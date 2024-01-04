// Copyright 2024 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package plugin implements CloudKMS plugin for GKE as described in go/gke-secrets-encryption-design.
package v2

import (
	"context"
	"encoding/base64"
	"fmt"
	"net"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/golang/glog"

	"google.golang.org/api/cloudkms/v1"
	"google.golang.org/grpc"

	"github.com/GoogleCloudPlatform/k8s-cloudkms-plugin/plugin"
)

const (
	netProtocol     = "unix"
	apiVersion      = "v2beta1"
	ok              = "ok"
	ping            = "cGluZw=="
	keyNotReachable = "Cloud KMS key is not reachable"
	keyDisabled     = "Cloud KMS key is not enabled or no cloudkms.cryptoKeys.get permission"
)

// Store the last known primary key version resource name
// to return as KeyId in case when the Cloud KMS service is not reachable
// because KeyId in StatusResponse cannot be empty and shouldn't trigger key migration
// in case of transient remote service unavailability
var lastKeyVersion string

// Regex to extract Cloud KMS key resource name from the key version resource name
var compRegEx = regexp.MustCompile(`projects\/[^/]+\/locations\/[^/]+\/keyRings\/[^/]+\/cryptoKeys\/[^/]+`)

type Plugin struct {
	plugin.PluginConfig
}

// New constructs Plugin.
func NewPlugin(keyService *cloudkms.ProjectsLocationsKeyRingsCryptoKeysService, keyURI, pathToUnixSocketFile string) *Plugin {
	lastKeyVersion = keyURI
	return &Plugin{
		plugin.PluginConfig{
			KeyService:       keyService,
			KeyURI:           keyURI,
			PathToUnixSocket: pathToUnixSocketFile,
		},
	}
}

// Status returns the version of KMS API version that plugin supports.
// Response also contains the status of the plugin, which is calculated as availability of the
// encryption key that the plugin is confinged with, and the current primary key version.
// kube-apiserver will provide this key version in Encrypt and Decrypt calls and will be able
// to know whether the remote CLoud KMS key has been rotated or not.
func (g *Plugin) Status(ctx context.Context, request *StatusRequest) (*StatusResponse, error) {
	defer plugin.RecordCloudKMSOperation("encrypt", time.Now())

	var response StatusResponse
	req := &cloudkms.EncryptRequest{Plaintext: ping}
	resp, err := g.KeyService.Encrypt(g.KeyURI, req).Context(ctx).Do()
	if err != nil {
		plugin.CloudKMSOperationalFailuresTotal.WithLabelValues("encrypt").Inc()
		response = StatusResponse{Version: apiVersion, Healthz: keyNotReachable, KeyId: lastKeyVersion}
	} else {
		lastKeyVersion = resp.Name
		response = StatusResponse{Version: apiVersion, Healthz: ok, KeyId: resp.Name}
	}

	glog.V(4).Infof("Status response: %s", response.Healthz)
	return &response, nil
}

// Encrypt encrypts payload provided by K8S API Server.
func (g *Plugin) Encrypt(ctx context.Context, request *EncryptRequest) (*EncryptResponse, error) {
	glog.V(4).Infof("Processing request for encryption %s using %s", request.Uid, g.KeyURI)
	defer plugin.RecordCloudKMSOperation("encrypt", time.Now())

	req := &cloudkms.EncryptRequest{Plaintext: base64.StdEncoding.EncodeToString(request.Plaintext)}
	resp, err := g.KeyService.Encrypt(g.KeyURI, req).Context(ctx).Do()
	if err != nil {
		plugin.CloudKMSOperationalFailuresTotal.WithLabelValues("encrypt").Inc()
		return nil, err
	}

	cipher, err := base64.StdEncoding.DecodeString(resp.Ciphertext)
	if err != nil {
		return nil, err
	}

	lastKeyVersion = resp.Name
	response := EncryptResponse{Ciphertext: []byte(cipher), KeyId: resp.Name}
	glog.V(4).Infof("Processed request for encryption %s using %s", request.Uid, resp.Name)
	return &response, nil
}

// Decrypt decrypts payload supplied by K8S API Server.
func (g *Plugin) Decrypt(ctx context.Context, request *DecryptRequest) (*DecryptResponse, error) {
	glog.V(4).Infof("Processing request for decryption %s using %s", request.Uid, request.KeyId)
	defer plugin.RecordCloudKMSOperation("decrypt", time.Now())

	req := &cloudkms.DecryptRequest{
		Ciphertext: base64.StdEncoding.EncodeToString(request.Ciphertext),
	}
	resp, err := g.KeyService.Decrypt(extractKeyVersion(request.KeyId), req).Context(ctx).Do()
	if err != nil {
		plugin.CloudKMSOperationalFailuresTotal.WithLabelValues("decrypt").Inc()
		return nil, err
	}

	plain, err := base64.StdEncoding.DecodeString(resp.Plaintext)
	if err != nil {
		return nil, fmt.Errorf("failed to decode from base64, error: %v", err)
	}

	response := DecryptResponse{Plaintext: []byte(plain)}
	return &response, nil
}

// Extracts the Cloud KMS key resource name from the key version resource name
func extractKeyVersion(keyVersionId string) string {
	return compRegEx.FindString(keyVersionId)
}

func (g *Plugin) setupRPCServer() error {
	if err := g.cleanSockFile(); err != nil {
		return err
	}

	listener, err := net.Listen(netProtocol, g.PathToUnixSocket)
	if err != nil {
		return fmt.Errorf("failed to start listener, error: %v", err)
	}
	g.Listener = listener
	glog.Infof("Listening on unix domain socket: %s", g.PathToUnixSocket)

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
	if strings.HasPrefix(g.PathToUnixSocket, "@") {
		return nil
	}

	err := os.Remove(g.PathToUnixSocket)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete the socket file, error: %v", err)
	}
	return nil
}
