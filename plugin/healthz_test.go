package plugin

import (
	"encoding/json"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"testing"
	"time"

	"google.golang.org/api/cloudkms/v1"
	"google.golang.org/api/googleapi"

	"github.com/phayes/freeport"
)

func TestHealthzServer(t *testing.T) {
	var (
		//positiveTestIAMResponse = &cloudkms.TestIamPermissionsResponse{
		//	Permissions: []string{
		//		"cloudkms.cryptoKeyVersions.useToDecrypt",
		//		"cloudkms.cryptoKeyVersions.useToEncrypt",
		//	},
		//	ServerResponse: googleapi.ServerResponse{
		//		HTTPStatusCode: http.StatusOK,
		//	},
		//}
		negativeTestIAMResponse = &cloudkms.TestIamPermissionsResponse{
			Permissions: []string{},
			ServerResponse: googleapi.ServerResponse{
				HTTPStatusCode: http.StatusOK,
			},
		}
	)

	testCases := []struct {
		desc     string
		query    string
		response []json.Marshaler
		want     int
	}{
		//{
		//	desc:     "Positive response for TestIAM, not pinging CloudKMS",
		//	response: []json.Marshaler{positiveTestIAMResponse},
		//	want:     http.StatusOK,
		//},
		{
			desc:     "Negative response for TestIAM, not pinging CloudKMS",
			response: []json.Marshaler{negativeTestIAMResponse},
			want:     http.StatusForbidden,
		},
		//{
		//	desc:     "Positive response for TestIAM, Positive ping from CloudKMS",
		//	query:    "ping-kms=true",
		//	response: []json.Marshaler{positiveTestIAMResponse, positiveEncryptResponse, positiveDecryptResponse},
		//	want:     http.StatusOK,
		//},
		//{
		//	desc:     "Positive response for TestIAM, Negative ping from CloudKMS",
		//	query:    "ping-kms=true",
		//	response: []json.Marshaler{positiveTestIAMResponse, negativeEncryptResponse},
		//	want:     http.StatusServiceUnavailable,
		//},
	}

	for _, testCase := range testCases {
		t.Run(testCase.desc, func(t *testing.T) {
			tt := setUpWithResponses(t, keyName, 0, testCase.response...)
			defer tt.tearDown()

			// Ensure that serving both Metrics and Healthz
			mustServeMetrics(t)

			healthzPort := mustServeHealthz(t, tt)

			u := url.URL{
				Scheme:   "http",
				Host:     net.JoinHostPort("localhost", strconv.FormatUint(uint64(healthzPort), 10)),
				Path:     "healthz",
				RawQuery: testCase.query,
			}
			gotStatus, gotBody := mustGetHealthz(t, u)
			if gotStatus != testCase.want {
				t.Fatalf("Got %d for healthz status, want %d, response: %q", gotStatus, testCase.want, gotBody)
			}
		})
	}
}

func mustGetHealthz(t *testing.T, url url.URL) (int, []byte) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, url.String(), nil)
	if err != nil {
		t.Fatalf("Unable to create http request: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Unable to contact healthz endpoint of master: %v", err)
	}
	defer resp.Body.Close()
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read the Body of request for %v, error %v", url, err)
	}
	return resp.StatusCode, b
}

func mustServeHealthz(t *testing.T, tt *pluginTestCase) int {
	t.Helper()

	p, err := freeport.GetFreePort()
	if err != nil {
		t.Fatalf("Failed to allocate a free port for healthz server, err: %v", err)
	}

	h := &HealthZ{
		KeyName:        tt.keyURI,
		KeyService:     tt.keyService,
		UnixSocketPath: tt.Plugin.pathToUnixSocket,
		CallTimeout:    5 * time.Second,
		ServingURL: &url.URL{
			Host: net.JoinHostPort("localhost", strconv.FormatUint(uint64(p), 10)),
			Path: "healthz",
		},
	}

	c := h.Serve()
	// Giving some time for healthz server to start while listening on the error channel.
	select {
	case err := <-c:
		t.Fatalf("received an error while starting healthz server, error channel: %v", err)
	case <-time.After(3 * time.Second):
	}

	return p
}
