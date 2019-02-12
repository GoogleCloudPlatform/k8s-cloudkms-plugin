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

// Package fakekubeapi supports integration testing of kms-plugin by faking K8S kube-apiserver.
package fakekubeapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/GoogleCloudPlatform/k8s-cloudkms-plugin/testutils/kmspluginclient"
	"github.com/golang/glog"
	"github.com/google/go-cmp/cmp"
	"github.com/phayes/freeport"

	msgspb "github.com/GoogleCloudPlatform/k8s-cloudkms-plugin/plugin"
	corev1 "k8s.io/api/core/v1"
)

var (
	secretsURLRegex = regexp.MustCompile(`/api/v1/namespaces/[a-z-]*/secrets/[a-z-]*`)
)

// Server fakes kube-apiserver.
type Server struct {
	srv        *httptest.Server
	port       int
	namespaces corev1.NamespaceList
	secrets    map[string][]corev1.Secret
	kms        *kmspluginclient.Client
	timeout    time.Duration

	mux            sync.Mutex
	secretsListLog []corev1.Secret
	secretsPutLog  []corev1.Secret
}

// Client returns *http.Client for the fake.
func (f *Server) Client() *http.Client {
	return f.srv.Client()
}

// URL returns URL on which the fake is expecting requests.
func (f *Server) URL() string {
	return f.srv.URL
}

// Close closes the underlying httptest.Server.
func (f *Server) Close() {
	f.srv.Close()
}

// ListSecretsRequestsEquals validates that the supplied Secrets are equal to all secrets
// processed by the server via http.Get.
func (f *Server) ListSecretsRequestsEquals(r []corev1.Secret) error {
	f.mux.Lock()
	defer f.mux.Unlock()

	if diff := cmp.Diff(f.secretsListLog, r); diff != "" {
		return fmt.Errorf("list log differs from expected: (-want +got)\n%s", diff)
	}

	return nil
}

// PutSecretsEquals validates that the supplied Secrets are equal to all
// secrets processed by the server via http.Put.
func (f *Server) PutSecretsEquals(r []corev1.Secret) error {
	f.mux.Lock()
	defer f.mux.Unlock()

	if diff := cmp.Diff(f.secretsPutLog, r); diff != "" {
		return fmt.Errorf("put log differs from expected: (-want +got)\n%s", diff)
	}

	return nil
}

// New constructs kube-apiserver fake.
// It is the responsibility of the caller to call Close.
func New(namespaces corev1.NamespaceList, secrets map[string][]corev1.Secret, port int, kmsClient *kmspluginclient.Client, timeout time.Duration) (*Server, error) {
	var err error
	if port == 0 {
		port, err = freeport.GetFreePort()
		if err != nil {
			return nil, fmt.Errorf("failed to allocate port for fake kube-apiserver, error: %v", err)
		}
	}

	s := &Server{
		namespaces: namespaces,
		secrets:    secrets,
		kms:        kmsClient,
		timeout:    timeout,
	}

	s.srv = httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			s.processGet(r.URL.EscapedPath(), w)
		case http.MethodPut:
			s.processPut(r, w)
		default:
			http.Error(w, fmt.Sprintf("unexpected http method %v", r.Method), http.StatusBadRequest)
		}
	}))

	l, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", port))
	if err != nil {
		return nil, fmt.Errorf("failed to listen on port %d, error: %v", port, err)
	}
	s.srv.Listener = l
	s.srv.Start()
	return s, nil
}

func (f *Server) recordSecretList(s []corev1.Secret) {
	f.mux.Lock()
	defer f.mux.Unlock()
	f.secretsListLog = append(f.secretsListLog, s...)
}

func (f *Server) recordSecretPut(s corev1.Secret) {
	f.mux.Lock()
	defer f.mux.Unlock()
	f.secretsPutLog = append(f.secretsPutLog, s)
}

func (f *Server) processPut(r *http.Request, w http.ResponseWriter) {
	ctx, cancel := context.WithTimeout(context.Background(), f.timeout)
	defer cancel()

	glog.Infof("Processing PUT request %v", r)
	if !secretsURLRegex.MatchString(r.URL.EscapedPath()) {
		http.Error(w, fmt.Sprintf("unexpected uri: %s", r.URL.EscapedPath()), http.StatusNotFound)
		return
	}

	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to read the body of the request, error: %v", err), http.StatusBadRequest)
		return
	}

	s := &corev1.Secret{}
	if err := json.Unmarshal(b, s); err != nil {
		http.Error(w, fmt.Sprintf("failed to unmarshal request, error: %v", err), http.StatusBadRequest)
		return
	}

	f.recordSecretPut(*s)

	glog.Infoln("Sending secret for encryption to kms-plugin.")
	if _, err := f.kms.Encrypt(ctx, &msgspb.EncryptRequest{Version: "v1beta1", Plain: b}); err != nil {
		m := fmt.Sprintf("failed to transform secret, error: %v", err)
		glog.Warning(m)
		http.Error(w, m, http.StatusServiceUnavailable)
		return
	}
	glog.Info("kms-plugin processed the encryption request.")

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(s); err != nil {
		http.Error(w, fmt.Sprintf("failed to write response for secret put, error: %v", err), http.StatusBadRequest)
		return
	}
}

func (f *Server) processGet(url string, w http.ResponseWriter) {
	glog.Infof("Processing Get request %s", url)
	// TODO(alextc) Check URL - is it actually a get/list request for a Secret?
	var response interface{}
	switch {
	case url == "/api/v1/namespaces":
		response = f.namespaces

		// Expect url to be of the following format: /api/v1/namespaces/default/secrets.
	case strings.HasSuffix(url, "/secrets"):
		urlParts := strings.Split(url, "/")
		if len(urlParts) != 6 {
			http.Error(w, fmt.Sprintf("unexpected format of url: %q, wanted len of 4, got %d, parts: %#v", url, len(urlParts), urlParts), http.StatusBadRequest)
			return
		}
		s, ok := f.secrets[urlParts[4]]
		if !ok {
			http.Error(w, fmt.Sprintf("invalid test data, request for %q, but namespace %s was not provided", url, urlParts[4]), http.StatusNotFound)
			return
		}

		response = corev1.SecretList{
			Items: s,
		}
		f.recordSecretList(s)

	default:
		http.Error(w, fmt.Sprintf("Was not expecting call to %q", url), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, fmt.Sprintf("failed to write response for request:%s, err: %v", url, err), http.StatusInternalServerError)
	}
}
