/*
Copyright 2018 Google LLC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

https://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package plugin implements CloudKMS plugin for GKE as described in go/gke-secrets-encryption-design.
package plugin

import (
	"context"
	"encoding/base64"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"github.com/golang/glog"

	"google.golang.org/api/cloudkms/v1"
	"google.golang.org/grpc"
)

const (
	netProtocol    = "unix"
	apiVersion     = "v1beta1"
	runtimeName    = "CloudKMS"
	runtimeVersion = "0.0.1"
)

// Plugin is a CloudKMS plugin for K8S.
type Plugin struct {
	keyService       *cloudkms.ProjectsLocationsKeyRingsCryptoKeysService
	keyURI           string
	pathToUnixSocket string
	// Embedding these only to shorten access to fields.
	net.Listener
	*grpc.Server
}

// New constructs Plugin.
func New(keyService *cloudkms.ProjectsLocationsKeyRingsCryptoKeysService, keyURI, pathToUnixSocketFile string) *Plugin {
	return &Plugin{
		keyService:       keyService,
		keyURI:           keyURI,
		pathToUnixSocket: pathToUnixSocketFile,
	}
}

// Version returns the version of KMS Plugin.
func (g *Plugin) Version(ctx context.Context, request *VersionRequest) (*VersionResponse, error) {
	return &VersionResponse{Version: apiVersion, RuntimeName: runtimeName, RuntimeVersion: runtimeVersion}, nil
}

// Encrypt encrypts payload provided by K8S API Server.
func (g *Plugin) Encrypt(ctx context.Context, request *EncryptRequest) (*EncryptResponse, error) {
	glog.V(4).Infoln("Processing request for encryption.")
	// TODO(immutablet) check the version of the request and issue a warning if the version is not what the plugin expects.
	defer recordCloudKMSOperation("encrypt", time.Now())

	req := &cloudkms.EncryptRequest{Plaintext: base64.StdEncoding.EncodeToString(request.Plain)}
	resp, err := g.keyService.Encrypt(g.keyURI, req).Context(ctx).Do()
	if err != nil {
		cloudKMSOperationalFailuresTotal.WithLabelValues("encrypt").Inc()
		return nil, err
	}

	cipher, err := base64.StdEncoding.DecodeString(resp.Ciphertext)
	if err != nil {
		return nil, err
	}

	return &EncryptResponse{Cipher: []byte(cipher)}, nil
}

// Decrypt decrypts payload supplied by K8S API Server.
func (g *Plugin) Decrypt(ctx context.Context, request *DecryptRequest) (*DecryptResponse, error) {
	glog.V(4).Infoln("Processing request for decryption.")
	// TODO(immutableT) check the version of the request and issue a warning if the version is not what the plugin expects.
	defer recordCloudKMSOperation("decrypt", time.Now())

	req := &cloudkms.DecryptRequest{
		Ciphertext: base64.StdEncoding.EncodeToString(request.Cipher),
	}
	resp, err := g.keyService.Decrypt(g.keyURI, req).Context(ctx).Do()
	if err != nil {
		cloudKMSOperationalFailuresTotal.WithLabelValues("decrypt").Inc()
		return nil, err
	}

	plain, err := base64.StdEncoding.DecodeString(resp.Plaintext)
	if err != nil {
		return nil, fmt.Errorf("failed to decode from base64, error: %v", err)
	}

	return &DecryptResponse{Plain: []byte(plain)}, nil
}

func (g *Plugin) setupRPCServer() error {
	if err := g.cleanSockFile(); err != nil {
		return err
	}

	listener, err := net.Listen(netProtocol, g.pathToUnixSocket)
	if err != nil {
		return fmt.Errorf("failed to start listener, error: %v", err)
	}
	g.Listener = listener
	glog.Infof("Listening on unix domain socket: %s", g.pathToUnixSocket)

	g.Server = grpc.NewServer()
	RegisterKeyManagementServiceServer(g.Server, g)

	return nil
}

// ServeKMSRequests starts gRPC server or dies.
func (g *Plugin) ServeKMSRequests() (*grpc.Server, chan error) {
	errorChan := make(chan error, 1)
	if err := g.setupRPCServer(); err != nil {
		errorChan <- err
		close(errorChan)
		return nil, errorChan
	}

	go func() {
		defer close(errorChan)
		errorChan <- g.Serve(g.Listener)
	}()

	return g.Server, errorChan
}

func (g *Plugin) cleanSockFile() error {
	// @ implies the use of Linux socket namespace - no file on disk and nothing to clean-up.
	if strings.HasPrefix(g.pathToUnixSocket, "@") {
		return nil
	}

	err := os.Remove(g.pathToUnixSocket)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete the socket file, error: %v", err)
	}
	return nil
}
