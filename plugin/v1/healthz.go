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

package v1

import (
	"context"
	"fmt"

	"github.com/GoogleCloudPlatform/k8s-cloudkms-plugin/plugin"
	"github.com/golang/glog"
	grpc "google.golang.org/grpc"
)

var _ plugin.HealthChecker = (*HealthChecker)(nil)

type HealthChecker struct{}

func NewHealthChecker() *HealthChecker {
	return &HealthChecker{}
}

func (h *HealthChecker) PingRPC(ctx context.Context, conn *grpc.ClientConn) error {
	client := NewKeyManagementServiceClient(conn)

	if _, err := client.Version(ctx, &VersionRequest{
		Version: "v1beta1",
	}); err != nil {
		return fmt.Errorf("failed to retrieve version from gRPC endpoint: %w", err)
	}

	glog.V(4).Infof("Successfully pinged gRPC")
	return nil
}

func (h *HealthChecker) PingKMS(ctx context.Context, conn *grpc.ClientConn) error {
	client := NewKeyManagementServiceClient(conn)

	encryptResponse, err := client.Encrypt(ctx, &EncryptRequest{
		Version: apiVersion,
		Plain:   []byte("secret"),
	})
	if err != nil {
		return fmt.Errorf("failed to ping KMS: %w", err)
	}

	if _, err = client.Decrypt(ctx, &DecryptRequest{
		Version: apiVersion,
		Cipher:  []byte(encryptResponse.Cipher),
	}); err != nil {
		return fmt.Errorf("failed to ping KMS: %w", err)
	}

	return nil
}
