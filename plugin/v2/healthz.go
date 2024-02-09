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

package v2

import (
	"net/url"
	"time"

	"context"
	"fmt"

	kmspb "google.golang.org/api/cloudkms/v1"
	grpc "google.golang.org/grpc"

	"github.com/golang/glog"
	"github.com/google/uuid"

	"github.com/GoogleCloudPlatform/k8s-cloudkms-plugin/plugin"
)

type HealthZ struct {
	*plugin.HealthZ
}

func NewHealthChecker(keyName string, keyService *kmspb.ProjectsLocationsKeyRingsCryptoKeysService,
	unixSocketPath string, callTimeout time.Duration, servingURL *url.URL) *HealthZ {

	hz := &HealthZ{
		HealthZ: &plugin.HealthZ{
			KeyName:        keyName,
			KeyService:     keyService,
			UnixSocketPath: unixSocketPath,
			CallTimeout:    callTimeout,
			ServingURL:     servingURL,
		},
	}
	hz.HealthZ.HealthChecker = hz
	return hz
}

func (h *HealthZ) PingRPC(ctx context.Context, keyManagementServiceClient any) error {
	var c KeyManagementServiceClient = keyManagementServiceClient.(KeyManagementServiceClient)

	r := &StatusRequest{}
	if _, err := c.Status(ctx, r); err != nil {
		return fmt.Errorf("failed to retrieve version from gRPC endpoint:%s, error: %v", h.UnixSocketPath, err)
	}

	glog.V(4).Infof("Successfully pinged gRPC via %s", h.UnixSocketPath)
	return nil
}

func (h *HealthZ) PingKMS(ctx context.Context, keyManagementServiceClient any) error {
	var c KeyManagementServiceClient = keyManagementServiceClient.(KeyManagementServiceClient)

	plainText := []byte("secret")

	encryptResponse, err := c.Encrypt(ctx, &EncryptRequest{
		Uid:       uuid.NewString(),
		Plaintext: []byte(plainText),
	})
	if err != nil {
		return fmt.Errorf("failed to ping KMS: %v", err)
	}

	_, err = c.Decrypt(context.Background(), &DecryptRequest{
		Uid:        uuid.NewString(),
		Ciphertext: []byte(encryptResponse.Ciphertext),
	})
	if err != nil {
		return fmt.Errorf("failed to ping KMS: %v", err)
	}

	return nil
}

func (h *HealthZ) NewKeyManagementServiceClient(cc *grpc.ClientConn) any {
	return NewKeyManagementServiceClient(cc)
}
