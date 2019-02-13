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
	"context"
	"fmt"

	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/golang/glog"

	kmspb "google.golang.org/api/cloudkms/v1"
	"google.golang.org/grpc"

	"k8s.io/apimachinery/pkg/util/sets"
)

// HealthZ types that encapsulates healthz functionality of kms-plugin.
// The following health checks are performed:
// 1. Getting version of the plugin - validates gRPC connectivity.
// 2. Asserting that the caller has encrypt and decrypt permissions on the crypto key.
type HealthZ struct {
	KeyName        string
	KeyService     *kmspb.ProjectsLocationsKeyRingsCryptoKeysService
	UnixSocketPath string
	CallTimeout    time.Duration
	ServingURL     *url.URL
}

// Serve creates http server for hosting healthz.
func (h *HealthZ) Serve() chan error {
	errorChan := make(chan error)
	mux := http.NewServeMux()
	mux.HandleFunc(fmt.Sprintf("/%s", h.ServingURL.EscapedPath()), func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), h.CallTimeout)
		defer cancel()

		connection, err := dialUnix(h.UnixSocketPath)
		if err != nil {
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		defer connection.Close()

		c := NewKeyManagementServiceClient(connection)

		if err := h.pingRPC(ctx, c); err != nil {
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
			return
		}

		if err := h.testIAMPermissions(); err != nil {
			http.Error(w, err.Error(), http.StatusForbidden)
			return
		}

		if r.FormValue("ping-kms") == "true" {
			if err := h.pingKMS(ctx, c); err != nil {
				http.Error(w, err.Error(), http.StatusServiceUnavailable)
				return
			}
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	go func() {
		defer close(errorChan)
		glog.Infof("Registering healthz listener at %v", h.ServingURL)
		errorChan <- http.ListenAndServe(h.ServingURL.Host, mux)
	}()

	return errorChan
}

func (h *HealthZ) pingRPC(ctx context.Context, c KeyManagementServiceClient) error {
	r := &VersionRequest{Version: "v1beta1"}
	if _, err := c.Version(ctx, r); err != nil {
		return fmt.Errorf("failed to retrieve version from gRPC endpoint:%s, error: %v", h.UnixSocketPath, err)
	}

	glog.V(4).Infof("Successfully pinged gRPC via %s", h.UnixSocketPath)
	return nil
}

func (h *HealthZ) testIAMPermissions() error {
	want := sets.NewString("cloudkms.cryptoKeyVersions.useToEncrypt", "cloudkms.cryptoKeyVersions.useToDecrypt")
	glog.Infof("Testing IAM permissions, want %v", want.List())

	req := &kmspb.TestIamPermissionsRequest{
		Permissions: want.List(),
	}

	resp, err := h.KeyService.TestIamPermissions(h.KeyName, req).Do()
	if err != nil {
		return fmt.Errorf("failed to test IAM Permissions on %s, %v", h.KeyName, err)
	}
	glog.Infof("Got permissions: %v from CloudKMS for key:%s", resp.Permissions, h.KeyName)

	got := sets.NewString(resp.Permissions...)
	diff := want.Difference(got)

	if diff.Len() != 0 {
		glog.Errorf("Failed to validate IAM Permissions on %s, diff: %v", h.KeyName, diff)
		return fmt.Errorf("missing %v IAM permissions on CryptoKey:%s", diff, h.KeyName)
	}

	glog.Infof("Successfully validated IAM Permissions on %s.", h.KeyName)
	return nil
}

func (h *HealthZ) pingKMS(ctx context.Context, c KeyManagementServiceClient) error {
	plainText := []byte("secret")

	encryptRequest := EncryptRequest{Version: apiVersion, Plain: []byte(plainText)}
	encryptResponse, err := c.Encrypt(ctx, &encryptRequest)

	if err != nil {
		return fmt.Errorf("failed to ping KMS: %v", err)
	}

	decryptRequest := DecryptRequest{Version: apiVersion, Cipher: []byte(encryptResponse.Cipher)}
	_, err = c.Decrypt(context.Background(), &decryptRequest)
	if err != nil {
		return fmt.Errorf("failed to ping KMS: %v", err)
	}

	return nil
}

func dialUnix(unixSocketPath string) (*grpc.ClientConn, error) {
	protocol, addr := "unix", unixSocketPath
	dialer := func(addr string, timeout time.Duration) (net.Conn, error) {
		return net.DialTimeout(protocol, addr, timeout)
	}
	return grpc.Dial(addr, grpc.WithInsecure(), grpc.WithDialer(dialer))
}
