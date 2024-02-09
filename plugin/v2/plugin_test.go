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

package v2

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"testing"
	"time"

	"google.golang.org/api/cloudkms/v1"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
	"google.golang.org/grpc"

	"github.com/GoogleCloudPlatform/k8s-cloudkms-plugin/plugin"
	"github.com/GoogleCloudPlatform/k8s-cloudkms-plugin/testutils/fakekms"
	"github.com/golang/protobuf/proto"
	"github.com/phayes/freeport"
	"github.com/prometheus/client_golang/prometheus"
	prometheuspb "github.com/prometheus/client_model/go"

	"github.com/stretchr/testify/assert"
)

const (
	keyName        = "projects/my-project/locations/us-east1/keyRings/my-key-ring/cryptoKeys/my-key"
	keyVersionName = keyName + "/cryptoKeyVersions/1"
	keySuffix      = "test"
	// Tests fake encryption by assuming that "foo" decrypts to "bar"
	// Zm9v is base64 encoded foo
	// YmFy is base64 encoded "bar"
	ciphertext = "YmFy"
	plaintext  = "Zm9v"
)

var (
	positiveEncryptResponse = &cloudkms.EncryptResponse{
		Ciphertext: ciphertext,
		Name:       keyName,
		ServerResponse: googleapi.ServerResponse{
			HTTPStatusCode: http.StatusOK,
		},
	}
	positiveDecryptResponse = &cloudkms.DecryptResponse{
		Plaintext: plaintext,
		ServerResponse: googleapi.ServerResponse{
			HTTPStatusCode: http.StatusOK,
		},
	}
	negativeEncryptResponse = &cloudkms.EncryptResponse{
		ServerResponse: googleapi.ServerResponse{
			HTTPStatusCode: http.StatusInternalServerError,
		},
	}
)

type pluginTestCase struct {
	*Plugin
	pluginRPCSrv *grpc.Server
	fakeKMSSrv   *fakekms.Server
}

func (p *pluginTestCase) tearDown() {
	p.pluginRPCSrv.GracefulStop()
	p.fakeKMSSrv.Close()
}

func setUp(t *testing.T, fakeKMSSrv *fakekms.Server, keyName string, keySuffix string) *pluginTestCase {
	t.Helper()

	s, err := os.CreateTemp("", "plugin-test")
	if err != nil {
		t.Fatalf("Failed to create socket file, error: %v", err)
	}

	ctx := context.Background()
	waitForPluginStart := 3 * time.Second
	fakeKMSKeyService, err := cloudkms.NewService(ctx,
		option.WithHTTPClient(fakeKMSSrv.Client()))
	if err != nil {
		t.Fatalf("failed to instantiate cloud kms httpClient: %v", err)
	}
	fakeKMSKeyService.BasePath = fakeKMSSrv.URL()
	plugin := NewPlugin(fakeKMSKeyService.Projects.Locations.KeyRings.CryptoKeys, keyName, keySuffix, s.Name())
	pluginRPCSrv, errChan := plugin.ServeKMSRequests()
	// Giving some time for plugin to start while listening on the error channel.
	select {
	case err := <-errChan:
		t.Fatalf("received an error on plugin's error channel: %v", err)
	case <-time.After(waitForPluginStart):
	}

	return &pluginTestCase{plugin, pluginRPCSrv, fakeKMSSrv}
}

func setUpWithResponses(t *testing.T, keyName string, keySuffix string, delay time.Duration, responses ...json.Marshaler) *pluginTestCase {
	t.Helper()
	fakeKMSSrv, err := fakekms.NewWithResponses(keyName, 0, delay, responses...)
	if err != nil {
		t.Fatalf("Failed to construct FakeKMS, error: %v", err)
	}
	return setUp(t, fakeKMSSrv, keyName, keySuffix)
}

func TestMain(m *testing.M) {
	rand.Seed(time.Now().UnixNano())
	os.Exit(m.Run())
}

func TestEncrypt(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		desc                string
		wantEncryptRequests []*cloudkms.EncryptRequest
		wantDecryptRequests []*cloudkms.DecryptRequest
		testFn              func(t *testing.T, p *Plugin)
		response            json.Marshaler
		keySuffix           string
	}{
		{
			desc:                "Encrypt",
			wantEncryptRequests: []*cloudkms.EncryptRequest{{Plaintext: plaintext}},
			testFn: func(t *testing.T, p *Plugin) {
				encryptRequest := EncryptRequest{Plaintext: []byte("foo")}
				if _, err := p.Encrypt(context.Background(), &encryptRequest); err != nil {
					t.Fatalf("Failure while submitting request %v, error %v", encryptRequest, err)
				}
			},
			response: positiveEncryptResponse,
		},
		{
			desc:                "Decrypt",
			wantDecryptRequests: []*cloudkms.DecryptRequest{{Ciphertext: ciphertext}},
			testFn: func(t *testing.T, p *Plugin) {
				decryptRequest := DecryptRequest{Ciphertext: []byte("bar"), KeyId: keyVersionName}
				if _, err := p.Decrypt(context.Background(), &decryptRequest); err != nil {
					t.Fatalf("Failure while submitting request %v, error %v", decryptRequest, err)
				}
			},
			response: positiveDecryptResponse,
		},
		{
			desc:                "Encrypt",
			wantEncryptRequests: []*cloudkms.EncryptRequest{{Plaintext: plaintext}},
			testFn: func(t *testing.T, p *Plugin) {
				encryptRequest := EncryptRequest{Plaintext: []byte("foo")}
				if _, err := p.Encrypt(context.Background(), &encryptRequest); err != nil {
					t.Fatalf("Failure while submitting request %v, error %v", encryptRequest, err)
				}
			},
			response:  positiveEncryptResponse,
			keySuffix: "test",
		},
		{
			desc:                "Decrypt",
			wantDecryptRequests: []*cloudkms.DecryptRequest{{Ciphertext: ciphertext}},
			testFn: func(t *testing.T, p *Plugin) {
				decryptRequest := DecryptRequest{Ciphertext: []byte("bar"), KeyId: keyVersionName}
				if _, err := p.Decrypt(context.Background(), &decryptRequest); err != nil {
					t.Fatalf("Failure while submitting request %v, error %v", decryptRequest, err)
				}
			},
			response:  positiveDecryptResponse,
			keySuffix: "test",
		},
	}

	for _, testCase := range testCases {

		t.Run(testCase.desc, func(t *testing.T) {
			t.Parallel()

			tt := setUpWithResponses(t, keyName, testCase.keySuffix, 0, testCase.response)
			t.Cleanup(func() {
				tt.tearDown()
			})

			testCase.testFn(t, tt.Plugin)

			if err := tt.fakeKMSSrv.EncryptRequestsEqual(testCase.wantEncryptRequests); err != nil {
				t.Fatalf("Failed to compare last processed request on KMS Server, error: %v", err)
			}

			if err := tt.fakeKMSSrv.DecryptRequestsEqual(testCase.wantDecryptRequests); err != nil {
				t.Fatalf("Failed to compare last processed request on KMS Server, error: %v", err)
			}
		})
	}
}

func TestGatherMetrics(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		desc      string
		testFn    func(t *testing.T, p *Plugin)
		want      []string
		response  json.Marshaler
		keySuffix string
	}{
		{
			desc: "Decrypt",
			testFn: func(t *testing.T, p *Plugin) {
				decryptRequest := DecryptRequest{Ciphertext: []byte("foo"), KeyId: keyVersionName}
				_, err := p.Decrypt(context.Background(), &decryptRequest)
				if err != nil {
					t.Fatal(err)
				}
			},
			want: []string{
				"roundtrip_latencies",
			},
			response: positiveDecryptResponse,
		},
		{
			desc: "Decrypt Failure",
			testFn: func(t *testing.T, p *Plugin) {
				decryptRequest := DecryptRequest{Ciphertext: []byte("foo"), KeyId: keyVersionName}
				_, err := p.Decrypt(context.Background(), &decryptRequest)
				if err == nil {
					t.Fatal("expected Decrypt to fail")
				}
			},
			want: []string{
				"roundtrip_latencies",
				"failures_count",
			},
			response: &cloudkms.DecryptResponse{
				ServerResponse: googleapi.ServerResponse{
					HTTPStatusCode: http.StatusInternalServerError,
				},
			},
		},
		{
			desc: "Encrypt",
			testFn: func(t *testing.T, p *Plugin) {
				encryptRequest := EncryptRequest{Plaintext: []byte("foo")}
				_, err := p.Encrypt(context.Background(), &encryptRequest)
				if err != nil {
					t.Fatal(err)
				}
			},
			want: []string{
				"roundtrip_latencies",
			},
			response: positiveEncryptResponse,
		},
		{
			desc: "Encrypt Failure",
			testFn: func(t *testing.T, p *Plugin) {
				encryptRequest := EncryptRequest{Plaintext: []byte("foo")}
				_, err := p.Encrypt(context.Background(), &encryptRequest)
				if err == nil {
					t.Fatal("expected Encrypt to fail")
				}
			},
			want: []string{
				"roundtrip_latencies",
				"failures_count",
			},
			response: negativeEncryptResponse,
		},
	}

	for _, testCase := range testCases {

		t.Run(testCase.desc, func(t *testing.T) {
			t.Parallel()

			tt := setUpWithResponses(t, keyName, testCase.keySuffix, 0, testCase.response)
			t.Cleanup(func() {
				tt.tearDown()
			})
			testCase.testFn(t, tt.Plugin)

			got, err := prometheus.DefaultGatherer.Gather()
			if err != nil {
				t.Fatalf("failed to gather metrics: %s", err)
			}
			checkForExpectedMetrics(t, got, testCase.want)
		})
	}
}

func TestKMSTimeout(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		desc          string
		response      json.Marshaler
		responseDelay time.Duration
		pluginTimeout time.Duration
		testFn        func(ctx context.Context, t *testing.T, p *Plugin)
		keySuffix     string
	}{
		{
			desc:          "Encrypt",
			response:      positiveEncryptResponse,
			pluginTimeout: 1 * time.Second,
			responseDelay: 3 * time.Second,
			testFn: func(ctx context.Context, t *testing.T, p *Plugin) {
				encryptRequest := EncryptRequest{Plaintext: []byte("foo")}
				if _, err := p.Encrypt(ctx, &encryptRequest); err == nil {
					t.Fatal("exected to timeout")
				}
			},
			keySuffix: "test",
		},
		{
			desc:          "Decrypt",
			response:      positiveDecryptResponse,
			pluginTimeout: 1 * time.Second,
			responseDelay: 3 * time.Second,
			testFn: func(ctx context.Context, t *testing.T, p *Plugin) {
				decryptRequest := DecryptRequest{Ciphertext: []byte("bar")}
				if _, err := p.Decrypt(ctx, &decryptRequest); err == nil {
					t.Fatal("exected to timeout")
				}
			},
			keySuffix: "test",
		},
	}

	for _, testCase := range testCases {

		t.Run(testCase.desc, func(t *testing.T) {
			t.Parallel()

			tt := setUpWithResponses(t, keyName, testCase.keySuffix, testCase.responseDelay, testCase.response)
			t.Cleanup(func() {
				tt.tearDown()
			})

			ctx, cancel := context.WithTimeout(context.Background(), testCase.pluginTimeout)
			t.Cleanup(func() {
				cancel()
			})
			testCase.testFn(ctx, t, tt.Plugin)
		})
	}
}

func TestSocket(t *testing.T) {
	t.Parallel()

	tt := setUpWithResponses(t, keyName, keySuffix, 0)
	t.Cleanup(func() {
		tt.tearDown()
	})

	fileInfo, err := os.Stat(tt.Plugin.PathToUnixSocket)
	if err != nil {
		t.Fatalf("failed to stat socket %q, error %v", tt.Plugin.PathToUnixSocket, err)
	}

	if (fileInfo.Mode() & os.ModeSocket) != os.ModeSocket {
		t.Fatalf("got %v, wanted Srwxr-xr-x", fileInfo.Mode())
	}

	tt.GracefulStop()

	if _, err := os.Stat(tt.Plugin.PathToUnixSocket); err == nil {
		t.Fatal("expected socket to be cleaned-up by now.")
	}
}

func TestMetricsServer(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	metricsPort := mustServeMetrics(t)

	tt := setUpWithResponses(t, keyName, keySuffix, 0, positiveEncryptResponse)
	t.Cleanup(func() {
		tt.tearDown()
	})

	encryptRequest := EncryptRequest{Plaintext: []byte("foo")}
	if _, err := tt.Plugin.Encrypt(ctx, &encryptRequest); err != nil {
		t.Fatalf("Failed to submit encrypt request to plugin, error %v", err)
	}

	m, err := scrapeMetrics(metricsPort)
	if err != nil {
		t.Fatalf("Failed to scrape metrics, %v", err)
	}
	checkForExpectedMetrics(t, m, []string{"roundtrip_latencies"})
}

func mustServeMetrics(t *testing.T) int {
	t.Helper()
	p, err := freeport.GetFreePort()
	if err != nil {
		t.Fatalf("Failed to allocate a free port for metrics server, err: %v", err)
	}

	m := &plugin.Metrics{
		ServingURL: &url.URL{
			Host: net.JoinHostPort("localhost", strconv.FormatUint(uint64(p), 10)),
			Path: "metrics",
		},
	}

	c := m.Serve()
	// Giving some time for metrics server to start while listening on the error channel.
	select {
	case err := <-c:
		t.Fatalf("received an error while starting metrics server, error channel: %v", err)
	// TODO (alextc): Instead of waiting re-try scrapeMetrics call.
	case <-time.After(5 * time.Second):
	}

	return p
}

// scrapeMetrics scrapes Prometheus metrics.
// From https://github.com/kubernetes/kubernetes/blob/master/test/integration/metrics/metrics_test.go#L40
func scrapeMetrics(port int) ([]*prometheuspb.MetricFamily, error) {
	var scrapeRequestHeader = "application/vnd.google.protobuf;proto=io.prometheus.client.MetricFamily;encoding=compact-text"
	u := url.URL{
		Scheme: "http",
		Host:   net.JoinHostPort("localhost", strconv.FormatUint(uint64(port), 10)),
		Path:   "metrics",
	}

	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("unable to create http request: %v", err)
	}
	// Ask the prometheus exporter for its text protocol buffer format, since it's
	// much easier to parse than its plain-text format. Don't use the serialized
	// proto representation since it uses a non-standard varint delimiter between
	// metric families.
	req.Header.Add("Accept", scrapeRequestHeader)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("unable to contact metrics endpoint of the master: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("non-200 response trying to scrape metrics from the master: %v", resp.Status)
	}

	// Each line in the response body should contain all the data for a single metric.
	var metrics []*prometheuspb.MetricFamily
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		var metric prometheuspb.MetricFamily
		if err := proto.UnmarshalText(scanner.Text(), &metric); err != nil {
			return nil, fmt.Errorf("failed to unmarshal line of metrics response: %v", err)
		}
		metrics = append(metrics, &metric)
	}
	return metrics, scanner.Err()
}

func checkForExpectedMetrics(t *testing.T, metrics []*prometheuspb.MetricFamily, expectedMetrics []string) {
	t.Helper()
	foundMetrics := make(map[string]bool)
	for _, metric := range metrics {
		foundMetrics[metric.GetName()] = true
	}
	for _, expected := range expectedMetrics {
		if _, found := foundMetrics[expected]; !found {
			t.Errorf("Master metrics did not include expected metric %q\n.Metrics:\n%v", expected, foundMetrics)
		}
	}
}

func TestExtractKeyVersion(t *testing.T) {
	tests := []struct {
		keyVersionId string
		expectedKey  string
	}{
		{
			keyVersionId: keyName + "/cryptoKeyVersions/123",
			expectedKey:  keyName,
		},
		{
			keyVersionId: keyName + "/cryptoKeyVersions/456",
			expectedKey:  keyName,
		},
		{
			keyVersionId: keyName + "/cryptoKeyVersions/123:test",
			expectedKey:  keyName,
		},
		{
			keyVersionId: keyName + ":test",
			expectedKey:  keyName,
		},
		{
			keyVersionId: keyName + "/cryptoKeyVersions/123:",
			expectedKey:  keyName,
		},
		{
			keyVersionId: keyName + ":",
			expectedKey:  keyName,
		},
		{
			keyVersionId: "projects/my-project",
			expectedKey:  "",
		},
		{
			keyVersionId: keyName,
			expectedKey:  keyName,
		},
		{
			keyVersionId: "1234567",
			expectedKey:  "",
		},
	}

	for _, test := range tests {
		actualKey := extractKeyName(test.keyVersionId)
		assert.Equal(t, test.expectedKey, actualKey)
	}
}
