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

// Package fakekms supports integration testing of kms-plugin by faking CloudKMS.
package fakekms

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"time"

	"github.com/golang/glog"
	"github.com/google/go-cmp/cmp"
	"github.com/phayes/freeport"
	"google.golang.org/api/cloudkms/v1"
)

// Server fakes CloudKMS.
type Server struct {
	srv               *httptest.Server
	mux               sync.Mutex
	encryptRequestLog []*cloudkms.EncryptRequest
	decryptRequestLog []*cloudkms.DecryptRequest
	iamTestRequestLog []*cloudkms.TestIamPermissionsRequest
	port              int
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

// EncryptRequestsEqual validates that the supplied EncryptRequests are equal to all
// EncryptRequests processed by the server.
func (f *Server) EncryptRequestsEqual(r []*cloudkms.EncryptRequest) error {
	f.mux.Lock()
	defer f.mux.Unlock()

	if diff := cmp.Diff(f.encryptRequestLog, r); diff != "" {
		return fmt.Errorf("EncryptRequests differs from expected:(-want +got)\n%s", diff)
	}

	return nil
}

// DecryptRequestsEqual validates that the supplied DecryptRequests are equal to the all
// DecryptRequests processed by the server.
func (f *Server) DecryptRequestsEqual(r []*cloudkms.DecryptRequest) error {
	f.mux.Lock()
	defer f.mux.Unlock()

	if diff := cmp.Diff(f.decryptRequestLog, r); diff != "" {
		return fmt.Errorf("DecryptRequests differ from expected: (-want +got)\n%s", diff)
	}

	return nil
}

// TestIAMRequestsEqual validates that the supplied TestIamPermissionsRequests are equal to the all
// TestIamPermissionsRequests processed by the server.
func (f *Server) TestIAMRequestsEqual(r []*cloudkms.TestIamPermissionsRequest) error {
	f.mux.Lock()
	defer f.mux.Unlock()

	if diff := cmp.Diff(f.iamTestRequestLog[len(f.iamTestRequestLog)-1], r); diff != "" {
		return fmt.Errorf("Last TestIAMPermissionsRequest differs from expected: (-want +got)\n%s", diff)
	}

	return nil
}

func (f *Server) recordEncryptRequest(r *cloudkms.EncryptRequest) {
	f.mux.Lock()
	defer f.mux.Unlock()
	f.encryptRequestLog = append(f.encryptRequestLog, r)
}

func (f *Server) recordDecryptRequest(r *cloudkms.DecryptRequest) {
	f.mux.Lock()
	defer f.mux.Unlock()
	f.decryptRequestLog = append(f.decryptRequestLog, r)
}

func (f *Server) recordTestIAMRequest(r *cloudkms.TestIamPermissionsRequest) {
	f.mux.Lock()
	defer f.mux.Unlock()
	f.iamTestRequestLog = append(f.iamTestRequestLog, r)
}

// NewWithPipethrough creates and returns *Server that simply passed through the requests, by
// replacing ciphertext to cleartext and vice versa.
// Callers are also responsible for calling Close after completing tests.
// keyName simulates CloudKMS' keyName and is taken into account when calculating expected URL endpoints.
func NewWithPipethrough(keyName string, port int) (*Server, error) {
	handle := func(req json.Marshaler) (json.Marshaler, int, error) {
		glog.Infof("Processing request: %#v", req, req)

		switch r := req.(type) {
		case *cloudkms.EncryptRequest:
			return &cloudkms.EncryptResponse{
				Name:       keyName,
				Ciphertext: r.Plaintext,
			}, http.StatusOK, nil
		case *cloudkms.DecryptRequest:
			return &cloudkms.DecryptResponse{
				Plaintext: r.Ciphertext,
			}, http.StatusOK, nil
		case *cloudkms.TestIamPermissionsRequest:
			return &cloudkms.TestIamPermissionsRequest{
				Permissions: []string{
					"cloudkms.cryptoKeyVersions.useToEncrypt",
					"cloudkms.cryptoKeyVersions.useToDecrypt",
				},
			}, http.StatusOK, nil
		default:
			return nil, http.StatusInternalServerError, fmt.Errorf("was not expecting request type:%T", r)
		}
	}

	return newWithCallback(keyName, port, 0, handle)
}

// NewWithResponses creates and returns *Server.
// It is the responsibility of the caller to supply the expected number of Responses.
// When the provided Responses are exhausted an error will be returned.
// Callers are also responsible for calling Close after completing tests.
// keyName simulates CloudKMS' keyName and is taken into account when calculating expected URL endpoints.
// delay allows the caller to simulate delayed responses from KMS.
func NewWithResponses(keyName string, port int, delay time.Duration, responses ...json.Marshaler) (*Server, error) {
	handle := func(req json.Marshaler) (json.Marshaler, int, error) {
		if len(responses) == 0 {
			return nil, http.StatusServiceUnavailable, errors.New("list of responses is empty")
		}

		status := http.StatusInternalServerError
		switch req.(type) {
		case *cloudkms.EncryptRequest:
			e, ok := responses[0].(*cloudkms.EncryptResponse)
			if !ok {
				return nil, status, errors.New("request for encrypt does not have a corresponding response of cloudkms.EncryptResponse")
			}
			status = e.HTTPStatusCode
		case *cloudkms.DecryptRequest:
			d, ok := responses[0].(*cloudkms.DecryptResponse)
			if !ok {
				return nil, status, errors.New("request for decrypt does not have a corresponding response of cloudkms.DecryptResponse")
			}
			status = d.HTTPStatusCode
		case *cloudkms.TestIamPermissionsRequest:
			t, ok := responses[0].(*cloudkms.TestIamPermissionsResponse)
			if !ok {
				return nil, status, errors.New("request for testIamPermissions does not have a corresponding response of cloudkms.TestIAMPermissionResponse")
			}
			status = t.HTTPStatusCode
		}
		r := responses[0]
		responses = responses[1:]
		return r, status, nil
	}

	return newWithCallback(keyName, port, delay, handle)
}

func newWithCallback(keyName string, port int, delay time.Duration, handle func(req json.Marshaler) (json.Marshaler, int, error)) (*Server, error) {
	var err error
	if port == 0 {
		port, err = freeport.GetFreePort()
		if err != nil {
			return nil, fmt.Errorf("failed to allocate port for fake kms, error: %v", err)
		}
	}

	s := &Server{port: port}
	s.srv = httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(delay)

		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			http.Error(w, fmt.Sprintf("can't read the body of the request, error: %v", err), http.StatusBadRequest)
			return
		}

		var (
			response json.Marshaler
			status   = http.StatusInternalServerError
		)
		switch r.URL.EscapedPath() {
		case fmt.Sprintf("/v1/%s:encrypt", keyName):
			e := &cloudkms.EncryptRequest{}
			if err := json.Unmarshal(body, e); err != nil {
				http.Error(w, err.Error(), status)
				return
			}
			s.recordEncryptRequest(e)

			response, status, err = handle(e)
			if err != nil {
				http.Error(w, err.Error(), status)
				return
			}
		case fmt.Sprintf("/v1/%s:decrypt", keyName):
			d := &cloudkms.DecryptRequest{}
			if err := json.Unmarshal(body, d); err != nil {
				http.Error(w, err.Error(), status)
				return
			}
			s.recordDecryptRequest(d)

			response, status, err = handle(d)
			if err != nil {
				http.Error(w, err.Error(), status)
				return
			}
		case fmt.Sprintf("/v1/%s:testIamPermissions", keyName):
			t := &cloudkms.TestIamPermissionsRequest{}
			if err := json.Unmarshal(body, t); err != nil {
				http.Error(w, err.Error(), status)
				return
			}
			s.recordTestIAMRequest(t)

			response, status, err = handle(t)
			if err != nil {
				http.Error(w, err.Error(), status)
				return
			}
		default:
			http.Error(w, fmt.Sprintf("Was not expecting call to %q", r.URL.EscapedPath()), status)
			return
		}

		w.WriteHeader(status)
		if err := json.NewEncoder(w).Encode(response); err != nil {
			http.Error(w, fmt.Sprintf("failed to marshal response, error %v", err), http.StatusInternalServerError)
			return
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
