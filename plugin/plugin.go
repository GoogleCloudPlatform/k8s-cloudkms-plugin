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

package plugin

import (
	"encoding/base64"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/golang/glog"
	k8spb "github.com/immutablet/k8s-cloudkms-plugin/v1beta1"

	"golang.org/x/net/context"
	"golang.org/x/sys/unix"

	cloudkms "google.golang.org/api/cloudkms/v1"
	"google.golang.org/grpc"
	"strings"
)

const (
	encryptIAMPermission = "cloudkms.cryptoKeyVersions.useToEncrypt"
	decryptIAMPermission = "cloudkms.cryptoKeyVersions.useToDecrypt"
)

func init() {
	RegisterMetrics()
}


// Plugin CloudKMS plugin for K8S.
type Plugin struct {
	keys             *cloudkms.ProjectsLocationsKeyRingsCryptoKeysService
	keyURI           string
	pathToUnixSocket string
	net.Listener
	*grpc.Server
}

// New constructs Plugin.
func New(keyURI, pathToUnixSocketFile, gceConfig string) (*Plugin, error) {
	httpClient, err := newHTTPClient(gceConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to instantiate http httpClient: %v", err)
	}

	kmsClient, err := cloudkms.New(httpClient)
	if err != nil {
		return nil, fmt.Errorf("failed to instantiate cloud kms httpClient: %v", err)
	}

	plugin := new(Plugin)
	plugin.keys = kmsClient.Projects.Locations.KeyRings.CryptoKeys
	plugin.keyURI = keyURI
	plugin.pathToUnixSocket = pathToUnixSocketFile
	return plugin, nil
}

// Stop stops Plugin.
func (g *Plugin) Stop() {
	if g.Server != nil {
		g.Server.Stop()
	}

	if g.Listener != nil {
		g.Listener.Close()
	}
}

// Version returns the version of KMS Plugin.
func (g *Plugin) Version(ctx context.Context, request *k8spb.VersionRequest) (*k8spb.VersionResponse, error) {
	return &k8spb.VersionResponse{Version: apiVersion, RuntimeName: runtime, RuntimeVersion: runtimeVersion}, nil
}

func (g *Plugin) mustServeKMSRequests() {

	err := g.setupRPCServer()
	if err != nil {
		glog.Fatalf("failed to setup gRPC Server, %v", err)
	}

	go g.mustServeRPC()
}

func (g *Plugin) mustServeRPC() {
	err := g.Serve(g.Listener)
	if err != nil {
		glog.Fatalf("failed to serve gRPC, %v", err)
	}
	glog.Infof("Serving gRPC")
}

// Encrypt encrypts payload provided by K8S API Server.
func (g *Plugin) Encrypt(ctx context.Context, request *k8spb.EncryptRequest) (*k8spb.EncryptResponse, error) {
	defer RecordCloudKMSOperation("encrypt", time.Now())
	glog.Infof("Processing EncryptRequest with keyURI: %s", g.keyURI)

	kmsEncryptRequest := &cloudkms.EncryptRequest{Plaintext: base64.StdEncoding.EncodeToString(request.Plain)}

	kmsEncryptResponse, err := g.keys.Encrypt(g.keyURI, kmsEncryptRequest).Do()
	if err != nil {
		cloudKMSOperationalFailuresTotal.WithLabelValues("encrypt").Inc()
		return nil, err
	}

	cipher, err := base64.StdEncoding.DecodeString(kmsEncryptResponse.Ciphertext)
	if err != nil {
		return nil, err
	}

	return &k8spb.EncryptResponse{Cipher: []byte(cipher)}, nil
}

// Decrypt decrypts payload supplied by K8S API Server.
func (g *Plugin) Decrypt(ctx context.Context, request *k8spb.DecryptRequest) (*k8spb.DecryptResponse, error) {
	defer RecordCloudKMSOperation("decrypt", time.Now())

	glog.Infof("Processing DecryptRequest with keyURI: %s", g.keyURI)

	kmsDecryptRequest := &cloudkms.DecryptRequest{
		Ciphertext: base64.StdEncoding.EncodeToString(request.Cipher),
	}

	kmsDecryptResponse, err := g.keys.Decrypt(g.keyURI, kmsDecryptRequest).Do()
	if err != nil {
		cloudKMSOperationalFailuresTotal.WithLabelValues("decrypt").Inc()
		return nil, err
	}

	plain, err := base64.StdEncoding.DecodeString(kmsDecryptResponse.Plaintext)
	if err != nil {
		return nil, err
	}

	return &k8spb.DecryptResponse{Plain: []byte(plain)}, nil
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
	k8spb.RegisterKeyManagementServiceServer(g.Server, g)

	return nil
}

func (g *Plugin) cleanSockFile() error {

	// @ implies the use of Linux socket namespace - no file on disk and nothing to clean-up.
	if strings.HasPrefix(g.pathToUnixSocket, "@") {
		return nil
	}

	err := unix.Unlink(g.pathToUnixSocket)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete the socket file, error: %v", err)
	}
	return nil
}

func (g *Plugin) newUnixSocketConnection() (*grpc.ClientConn, error) {
	protocol, addr := "unix", g.pathToUnixSocket
	dialer := func(addr string, timeout time.Duration) (net.Conn, error) {
		return net.DialTimeout(protocol, addr, timeout)
	}
	connection, err := grpc.Dial(addr, grpc.WithInsecure(), grpc.WithDialer(dialer))
	if err != nil {
		return nil, err
	}

	return connection, nil
}
