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

// Implementation of the KMS Plugin API v1.
package v1

import (
	"context"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/golang/glog"

	"google.golang.org/api/cloudkms/v1"
	grpc "google.golang.org/grpc"

	"github.com/GoogleCloudPlatform/k8s-cloudkms-plugin/plugin"
)

const (
	apiVersion     = "v1beta1"
	runtimeName    = "CloudKMS"
	runtimeVersion = "0.0.1"
)

var _ plugin.Plugin = (*Plugin)(nil)

// Plugin is the v1 implementation of a plugin.
type Plugin struct {
	keyService *cloudkms.ProjectsLocationsKeyRingsCryptoKeysService
	keyURI     string
}

// NewPlugin creates a new v1 plugin
func NewPlugin(keyService *cloudkms.ProjectsLocationsKeyRingsCryptoKeysService, keyURI string) *Plugin {
	return &Plugin{
		keyService: keyService,
		keyURI:     keyURI,
	}
}

// Register registers the plugin as a service management service.
func (g *Plugin) Register(s *grpc.Server) {
	RegisterKeyManagementServiceServer(s, g)
}

// Version returns the version of KMS Plugin.
func (g *Plugin) Version(ctx context.Context, request *VersionRequest) (*VersionResponse, error) {
	return &VersionResponse{
		Version:        apiVersion,
		RuntimeName:    runtimeName,
		RuntimeVersion: runtimeVersion,
	}, nil
}

// Encrypt encrypts payload provided by K8S API Server.
func (g *Plugin) Encrypt(ctx context.Context, request *EncryptRequest) (*EncryptResponse, error) {
	glog.V(4).Infoln("Processing request for encryption.")
	defer plugin.RecordCloudKMSOperation("encrypt", time.Now().UTC())

	resp, err := g.keyService.Encrypt(g.keyURI, &cloudkms.EncryptRequest{
		Plaintext: base64.StdEncoding.EncodeToString(request.Plain),
	}).Context(ctx).Do()
	if err != nil {
		plugin.CloudKMSOperationalFailuresTotal.WithLabelValues("encrypt").Inc()
		return nil, err
	}

	cipher, err := base64.StdEncoding.DecodeString(resp.Ciphertext)
	if err != nil {
		return nil, err
	}

	return &EncryptResponse{
		Cipher: cipher,
	}, nil
}

// Decrypt decrypts payload supplied by K8S API Server.
func (g *Plugin) Decrypt(ctx context.Context, request *DecryptRequest) (*DecryptResponse, error) {
	glog.V(4).Infoln("Processing request for decryption.")
	defer plugin.RecordCloudKMSOperation("decrypt", time.Now().UTC())

	resp, err := g.keyService.Decrypt(g.keyURI, &cloudkms.DecryptRequest{
		Ciphertext: base64.StdEncoding.EncodeToString(request.Cipher),
	}).Context(ctx).Do()
	if err != nil {
		plugin.CloudKMSOperationalFailuresTotal.WithLabelValues("decrypt").Inc()
		return nil, err
	}

	plain, err := base64.StdEncoding.DecodeString(resp.Plaintext)
	if err != nil {
		return nil, fmt.Errorf("failed to decode from base64, error: %w", err)
	}

	return &DecryptResponse{
		Plain: plain,
	}, nil
}
