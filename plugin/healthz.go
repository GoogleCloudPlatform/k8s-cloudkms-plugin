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
	"google.golang.org/grpc/credentials/insecure"
	"k8s.io/apimachinery/pkg/util/sets"

	"net"
	"net/http"

	"github.com/golang/glog"
	"google.golang.org/grpc"
)

// HealthCheckerManager types that encapsulates healthz functionality of kms-plugin.
// The following health checks are performed:
// 1. Getting version of the plugin - validates gRPC connectivity.
// 2. Asserting that the caller has encrypt and decrypt permissions on the crypto key.
type HealthCheckerManager struct {
	keyName        string
	KeyService     *kmspb.ProjectsLocationsKeyRingsCryptoKeysService
	unixSocketPath string
	callTimeout    time.Duration
	servingURL     *url.URL

	plugin HealthChecker
}

type HealthChecker interface {
	PingRPC(context.Context, *grpc.ClientConn) error
	PingKMS(context.Context, *grpc.ClientConn) error
}

func NewHealthChecker(plugin HealthChecker, keyName string, keyService *kmspb.ProjectsLocationsKeyRingsCryptoKeysService,
	unixSocketPath string, callTimeout time.Duration, servingURL *url.URL) *HealthCheckerManager {

	return &HealthCheckerManager{
		keyName:        keyName,
		KeyService:     keyService,
		unixSocketPath: unixSocketPath,
		callTimeout:    callTimeout,
		servingURL:     servingURL,
	}
}

// Serve creates http server for hosting healthz.
func (m *HealthCheckerManager) Serve() chan error {
	errorCh := make(chan error)
	mux := http.NewServeMux()
	mux.HandleFunc(fmt.Sprintf("/%s", m.servingURL.EscapedPath()), m.HandlerFunc)

	go func() {
		defer close(errorCh)
		glog.Infof("Registering healthz listener at %v", m.servingURL)
		select {
		case errorCh <- http.ListenAndServe(m.servingURL.Host, mux):
		default:
		}
	}()

	return errorCh
}

func (m *HealthCheckerManager) HandlerFunc(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), m.callTimeout)
	defer cancel()

	conn, err := dialUnix(m.unixSocketPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	defer conn.Close()

	if err := m.plugin.PingRPC(ctx, conn); err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}

	if err := m.TestIAMPermissions(); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	if r.FormValue("ping-kms") == "true" {
		if err := m.plugin.PingKMS(ctx, conn); err != nil {
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

func (h *HealthCheckerManager) TestIAMPermissions() error {
	want := sets.NewString("cloudkms.cryptoKeyVersions.useToEncrypt", "cloudkms.cryptoKeyVersions.useToDecrypt")
	glog.Infof("Testing IAM permissions, want %v", want.List())

	req := &kmspb.TestIamPermissionsRequest{
		Permissions: want.List(),
	}

	resp, err := h.KeyService.TestIamPermissions(h.keyName, req).Do()
	if err != nil {
		return fmt.Errorf("failed to test IAM Permissions on %s, %v", h.keyName, err)
	}
	glog.Infof("Got permissions: %v from CloudKMS for key:%s", resp.Permissions, h.keyName)

	got := sets.NewString(resp.Permissions...)
	diff := want.Difference(got)

	if diff.Len() != 0 {
		glog.Errorf("Failed to validate IAM Permissions on %s, diff: %v", h.keyName, diff)
		return fmt.Errorf("missing %v IAM permissions on CryptoKey:%s", diff, h.keyName)
	}

	glog.Infof("Successfully validated IAM Permissions on %s.", h.keyName)
	return nil
}

func dialUnix(unixSocketPath string) (*grpc.ClientConn, error) {
	protocol, addr := "unix", unixSocketPath
	dialer := func(ctx context.Context, addr string) (net.Conn, error) {
		if deadline, ok := ctx.Deadline(); ok {
			return net.DialTimeout(protocol, addr, time.Until(deadline))
		}
		return net.DialTimeout(protocol, addr, 0)
	}

	return grpc.Dial(addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(dialer))
}
