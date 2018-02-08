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

	"github.com/golang/glog"

	"golang.org/x/net/context"
	"golang.org/x/oauth2/google"

	"google.golang.org/api/cloudkms/v1"

	k8spb "github.com/immutablet/k8s-kms-plugin/v1beta1"
	"golang.org/x/sys/unix"
	"google.golang.org/grpc"
	"net"
	"os"
	"time"
)

const (
	// Unix Domain Socket
	netProtocol    = "unix"
	version        = "v1beta1"
	runtime        = "CloudKMS"
	runtimeVersion = "0.0.1"
)

func init() {
	RegisterMetrics()
}

type Plugin struct {
	keys             *cloudkms.ProjectsLocationsKeyRingsCryptoKeysService
	keyURI           string
	pathToUnixSocket string
	net.Listener
	*grpc.Server
}

func New(projectID, locationID, keyRingID, keyID, pathToUnixSocketFile string) (*Plugin, error) {
	ctx := context.Background()
	client, err := google.DefaultClient(ctx, cloudkms.CloudPlatformScope)
	if err != nil {
		return nil, fmt.Errorf("failed to instantiate cloud sdk client: %v", err)
	}

	kmsClient, err := cloudkms.New(client)
	if err != nil {
		return nil, fmt.Errorf("failed to instantiate cloud kms client: %v", err)
	}

	keyUri := fmt.Sprintf("projects/%s/locations/%s/keyRings/%s/cryptoKeys/%s",
		projectID, locationID, keyRingID, keyID)

	plugin := new(Plugin)
	plugin.keys = kmsClient.Projects.Locations.KeyRings.CryptoKeys
	plugin.keyURI = keyUri
	plugin.pathToUnixSocket = pathToUnixSocketFile
	return plugin, nil
}

func (g *Plugin) SetupRPCServer() error {
	if err := g.cleanSockFile(); err != nil {
		return err
	}

	listener, err := net.Listen(netProtocol, g.pathToUnixSocket)
	if err != nil {
		return fmt.Errorf("failed to start listener, error: %v", err)
	}
	g.Listener = listener

	g.Server = grpc.NewServer()
	k8spb.RegisterKMSServiceServer(g.Server, g)

	return nil
}

func (g *Plugin) Stop() {
	if g.Server != nil {
		g.Server.Stop()
	}

	if g.Listener != nil {
		g.Listener.Close()
	}

	if _, err := os.Stat(g.pathToUnixSocket); err == nil {
		g.cleanSockFile()
	}
}

func (g *Plugin) Version(ctx context.Context, request *k8spb.VersionRequest) (*k8spb.VersionResponse, error) {
	return &k8spb.VersionResponse{Version: version, RuntimeName: runtime, RuntimeVersion: runtimeVersion}, nil
}

func (g *Plugin) Encrypt(ctx context.Context, request *k8spb.EncryptRequest) (*k8spb.EncryptResponse, error) {
	defer RecordCloudKMSOperation("encrypt", time.Now())
	glog.Infof("Processing EncryptRequest with keyURI: %s", g.keyURI)

	kmsEncryptRequest := &cloudkms.EncryptRequest{Plaintext: base64.StdEncoding.EncodeToString(request.Plain)}

	kmsEncryptResponse, err := g.keys.Encrypt(g.keyURI, kmsEncryptRequest).Do()
	if err != nil {
		return nil, err
	}

	cipher, err := base64.StdEncoding.DecodeString(kmsEncryptResponse.Ciphertext)
	if err != nil {
		return nil, err
	}

	return &k8spb.EncryptResponse{Cipher: []byte(cipher)}, nil
}

func (g *Plugin) Decrypt(ctx context.Context, request *k8spb.DecryptRequest) (*k8spb.DecryptResponse, error) {
	defer RecordCloudKMSOperation("decrypt", time.Now())

	glog.Infof("Processing DecryptRequest with keyURI: %s", g.keyURI)

	kmsDecryptRequest := &cloudkms.DecryptRequest{
		Ciphertext: base64.StdEncoding.EncodeToString(request.Cipher),
	}

	kmsDecryptResponse, err := g.keys.Decrypt(g.keyURI, kmsDecryptRequest).Do()
	if err != nil {
		return nil, err
	}

	plain, err := base64.StdEncoding.DecodeString(kmsDecryptResponse.Plaintext)
	if err != nil {
		return nil, err
	}

	return &k8spb.DecryptResponse{Plain: []byte(plain)}, nil
}

func (g *Plugin) cleanSockFile() error {
	err := unix.Unlink(g.pathToUnixSocket)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete the socket file, error: %v", err)
	}
	return nil
}
