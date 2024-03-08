// Copyright 2018 Google LLC
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

// Implementation of the KMS Plugin API v2.
package v2

import (
	"regexp"
	"sync"

	"google.golang.org/api/cloudkms/v1"
	grpc "google.golang.org/grpc"

	"context"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/GoogleCloudPlatform/k8s-cloudkms-plugin/plugin"
	"github.com/golang/glog"
)

const (
	apiVersion      = "v2beta1"
	ok              = "ok"
	ping            = "cGluZw=="
	keyNotReachable = "Cloud KMS key is not reachable"
	keyDisabled     = "Cloud KMS key is not enabled or no cloudkms.cryptoKeys.get permission"
)

// Regex to extract Cloud KMS key resource name from the key version resource name
var keyResourceRegEx = regexp.MustCompile(`projects\/[^/]+\/locations\/[^/]+\/keyRings\/[^/]+\/cryptoKeys\/[^/:]+`)

var _ plugin.Plugin = (*Plugin)(nil)

type Plugin struct {
	keyService *cloudkms.ProjectsLocationsKeyRingsCryptoKeysService
	keyURI     string
	keySuffix  string

	// lastKeyID stores the last known primary key version resource name to return
	// as KeyId in case when the Cloud KMS service is not reachable because KeyId
	// in StatusResponse cannot be empty and shouldn't trigger key migration in
	// case of transient remote service unavailability.
	lastKeyID     string
	lastKeyIDLock sync.RWMutex
}

// New constructs Plugin.
func NewPlugin(keyService *cloudkms.ProjectsLocationsKeyRingsCryptoKeysService, keyURI, keySuffix string) *Plugin {
	p := &Plugin{
		keyService: keyService,
		keyURI:     keyURI,
		keySuffix:  keySuffix,
	}
	p.setKeyID(keyURI)

	return p
}

// Register registers the plugin as a service management service.
func (g *Plugin) Register(s *grpc.Server) {
	RegisterKeyManagementServiceServer(s, g)
}

// Status returns the version of KMS API version that plugin supports.
// Response also contains the status of the plugin, which is calculated as availability of the
// encryption key that the plugin is confinged with, and the current primary key version.
// kube-apiserver will provide this key version in Encrypt and Decrypt calls and will be able
// to know whether the remote CLoud KMS key has been rotated or not.
func (g *Plugin) Status(ctx context.Context, request *StatusRequest) (*StatusResponse, error) {
	defer plugin.RecordCloudKMSOperation("encrypt", time.Now().UTC())

	keyID := g.keyID()

	statusResp := &StatusResponse{
		Version: apiVersion,
		KeyId:   keyID,
		Healthz: ok,
	}
	resp, err := g.keyService.Encrypt(g.keyURI, &cloudkms.EncryptRequest{
		Plaintext: ping,
	}).Context(ctx).Do()
	if err != nil {
		plugin.CloudKMSOperationalFailuresTotal.WithLabelValues("encrypt").Inc()
		statusResp.Healthz = keyNotReachable
	} else {
		g.setKeyID(resp.Name)
	}

	glog.V(4).Infof("Status response: %s", statusResp.Healthz)
	return statusResp, nil
}

// Encrypt encrypts payload provided by K8S API Server.
func (g *Plugin) Encrypt(ctx context.Context, request *EncryptRequest) (*EncryptResponse, error) {
	glog.V(4).Infof("Processing request for encryption %s using %s", request.Uid, g.keyURI)
	defer plugin.RecordCloudKMSOperation("encrypt", time.Now().UTC())

	resp, err := g.keyService.Encrypt(g.keyURI, &cloudkms.EncryptRequest{
		Plaintext: base64.StdEncoding.EncodeToString(request.Plaintext),
	}).Context(ctx).Do()
	if err != nil {
		plugin.CloudKMSOperationalFailuresTotal.WithLabelValues("encrypt").Inc()
		return nil, err
	}

	cipher, err := base64.StdEncoding.DecodeString(resp.Ciphertext)
	if err != nil {
		return nil, err
	}

	keyID := g.setKeyID(resp.Name)

	glog.V(4).Infof("Processed request for encryption %s using %s",
		request.Uid, keyID)

	return &EncryptResponse{
		Ciphertext: cipher,
		KeyId:      keyID,
	}, nil
}

// Decrypt decrypts payload supplied by K8S API Server.
func (g *Plugin) Decrypt(ctx context.Context, request *DecryptRequest) (*DecryptResponse, error) {
	glog.V(4).Infof("Processing request for decryption %s using %s", request.Uid, request.KeyId)
	defer plugin.RecordCloudKMSOperation("decrypt", time.Now().UTC())

	keyResourceName := g.keyURI
	if request.KeyId != "" { // request.KeyId is empty when health checker calls this method from PingKMS()
		keyResourceName = extractKeyName(request.KeyId)
	}
	resp, err := g.keyService.Decrypt(keyResourceName, &cloudkms.DecryptRequest{
		Ciphertext: base64.StdEncoding.EncodeToString(request.Ciphertext),
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
		Plaintext: plain,
	}, nil
}

// keyID is a threadsafe way to get the current key ID.
func (g *Plugin) keyID() string {
	g.lastKeyIDLock.RLock()
	defer g.lastKeyIDLock.RUnlock()
	return g.lastKeyID
}

// If the key id suffix has been passed in the command line parameters this
// function will return a key id value constructed from the Cloud KMS key
// version with appended suffix separated by ":"
//
// This is to return a unique key id to Kubernetes in case if the plugin is
// reconfigured to use a Cloud KMS key version which has been already in use
// before
func (g *Plugin) setKeyID(name string) string {
	result := name
	if v := g.keySuffix; v != "" {
		result = result + ":" + v
	}

	g.lastKeyIDLock.Lock()
	defer g.lastKeyIDLock.Unlock()
	g.lastKeyID = result
	return result
}

// Extracts the Cloud KMS key resource name from the key version resource name
func extractKeyName(keyVersionId string) string {
	return keyResourceRegEx.FindString(keyVersionId)
}
