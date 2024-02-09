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

package plugin

import (
	"net/url"
	"time"

	"context"
	"fmt"

	kmspb "google.golang.org/api/cloudkms/v1"
	"k8s.io/apimachinery/pkg/util/sets"

	"net"
	"net/http"

	"github.com/golang/glog"
	"google.golang.org/grpc"
)

// HealthZ types that encapsulates healthz functionality of kms-plugin.
// The following health checks are performed:
// 1. Getting version of the plugin - validates gRPC connectivity.
// 2. Asserting that the caller has encrypt and decrypt permissions on the crypto key.
type HealthZ struct {
	HealthChecker
	KeyName        string
	KeyService     *kmspb.ProjectsLocationsKeyRingsCryptoKeysService
	UnixSocketPath string
	CallTimeout    time.Duration
	ServingURL     *url.URL
}

type HealthChecker interface {
	Serve() chan error
	HandlerFunc(w http.ResponseWriter, r *http.Request)
	NewKeyManagementServiceClient(*grpc.ClientConn) any
	PingRPC(ctx context.Context, c any) error
	TestIAMPermissions() error
	PingKMS(ctx context.Context, c any) error
}

func NewHealthChecker(keyName string, keyService *kmspb.ProjectsLocationsKeyRingsCryptoKeysService,
	unixSocketPath string, callTimeout time.Duration, servingURL *url.URL) *HealthZ {

	return &HealthZ{
		KeyName:        keyName,
		KeyService:     keyService,
		UnixSocketPath: unixSocketPath,
		CallTimeout:    callTimeout,
		ServingURL:     servingURL,
	}

}

// Serve creates http server for hosting healthz.
func (h *HealthZ) Serve() chan error {
	errorCh := make(chan error)
	mux := http.NewServeMux()
	mux.HandleFunc(fmt.Sprintf("/%s", h.ServingURL.EscapedPath()), h.HandlerFunc)

	go func() {
		defer close(errorCh)
		glog.Infof("Registering healthz listener at %v", h.ServingURL)
		select {
		case errorCh <- http.ListenAndServe(h.ServingURL.Host, mux):
		default:
		}

	}()

	return errorCh
}

func (h *HealthZ) HandlerFunc(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), h.CallTimeout)
	defer cancel()

	connection, err := DialUnix(h.UnixSocketPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	defer connection.Close()

	c := h.NewKeyManagementServiceClient(connection)

	if err := h.PingRPC(ctx, c); err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}

	if err := h.TestIAMPermissions(); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	if r.FormValue("ping-kms") == "true" {
		if err := h.PingKMS(ctx, c); err != nil {
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

func (h *HealthZ) TestIAMPermissions() error {
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

func DialUnix(unixSocketPath string) (*grpc.ClientConn, error) {
	protocol, addr := "unix", unixSocketPath
	dialer := func(addr string, timeout time.Duration) (net.Conn, error) {
		return net.DialTimeout(protocol, addr, timeout)
	}
	return grpc.Dial(addr, grpc.WithInsecure(), grpc.WithDialer(dialer))
}
