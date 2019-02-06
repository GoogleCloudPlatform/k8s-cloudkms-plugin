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
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"golang.org/x/net/context"

	cloudkms "google.golang.org/api/cloudkms/v1"

	"github.com/golang/glog"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	expectedMetrics = []string{
		"apiserver_kms_kms_plugin_roundtrip_latencies",
		// "apiserver_kms_kms_plugin_failures_total",
		"go_memstats_alloc_bytes_total",
		"go_memstats_frees_total",
		"process_cpu_seconds_total",
	}
)

// validator checks plugin's pre-conditions.
type validator struct {
	*Plugin
}

// newValidator constructs Validator.
func newValidator(plugin *Plugin) *validator {
	return &validator{plugin}
}

func (v *validator) mustValidatePrerequisites() {
	v.mustHaveIAMPermissions()
	v.mustPingKMS()
}

func (v *validator) mustHaveIAMPermissions() {
	glog.Infof("Validating IAM Permissions on %s", v.keyURI)

	req := &cloudkms.TestIamPermissionsRequest{
		Permissions: []string{encryptIAMPermission, decryptIAMPermission},
	}

	resp, err := v.keys.TestIamPermissions(v.keyURI, req).Do()

	if err != nil {
		glog.Fatalf("Failed to test IAM Permissions on %s, %v", v.keyURI, err)
	}

	if !contains(resp.Permissions, encryptIAMPermission) {
		glog.Fatalf("Caller missing %s IAM Permission on %s", encryptIAMPermission, v.keyURI)
	}

	if !contains(resp.Permissions, decryptIAMPermission) {
		glog.Fatalf("Caller missing %s IAM Permission on %s", decryptIAMPermission, v.keyURI)
	}

	glog.Infof("Successfully validated IAM Permissions on %s.", v.keyURI)
}

func (v *validator) mustPingKMS() {
	plainText := []byte("secret")

	glog.Infof("Pinging KMS.")

	encryptRequest := EncryptRequest{Version: apiVersion, Plain: []byte(plainText)}
	encryptResponse, err := v.Encrypt(context.Background(), &encryptRequest)

	if err != nil {
		glog.Fatalf("failed to ping KMS: %v", err)
	}

	decryptRequest := DecryptRequest{Version: apiVersion, Cipher: []byte(encryptResponse.Cipher)}
	decryptResponse, err := v.Decrypt(context.Background(), &decryptRequest)
	if err != nil {
		glog.Fatalf("failed to ping KMS: %v", err)
	}

	if string(decryptResponse.Plain) != string(plainText) {
		glog.Fatalf("failed to ping kms, expected secret, but got %s", string(decryptResponse.Plain))
	}

	glog.Infof("Successfully pinged KMS.")
}

func (v *validator) mustPingRPC() {
	glog.Infof("Pinging KMS gRPC.")

	connection, err := v.newUnixSocketConnection()
	if err != nil {
		glog.Fatalf("failed to open unix socket, %v", err)
	}
	client := NewKeyManagementServiceClient(connection)

	plainText := []byte("secret")

	encryptRequest := EncryptRequest{Version: apiVersion, Plain: []byte(plainText)}
	encryptResponse, err := client.Encrypt(context.Background(), &encryptRequest)

	if err != nil {
		glog.Fatalf("failed to ping KMS: %v", err)
	}

	decryptRequest := DecryptRequest{Version: apiVersion, Cipher: []byte(encryptResponse.Cipher)}
	decryptResponse, err := client.Decrypt(context.Background(), &decryptRequest)
	if err != nil {
		glog.Fatalf("failed to ping KMS gRPC: %v", err)
	}

	if string(decryptResponse.Plain) != string(plainText) {
		glog.Fatalf("failed to ping KMS gRPC, expected secret, but got %s", string(decryptResponse.Plain))
	}

	glog.Infof("Successfully pinged gRPC KMS.")
}

func mustEmitOKHealthz() {
	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1%s%s", HealthzPort, HealthzPath))
	if err != nil {
		glog.Fatalf("Failed to reach %s%s: %v", HealthzPort, HealthzPath, err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)

	if !strings.Contains(string(body), "ok") {
		glog.Fatalf("Expected %s, but got %s", "ok", string(body))
	}
	glog.Infof("Declared Healthz to be OK on http://127.0.0.1%s%s.", HealthzPort, HealthzPath)
}

func mustEmitMetrics() {
	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1%s%s", MetricsPort, MetricsPath))
	if err != nil {
		glog.Fatalf("Failed to reach %s%s: %v", MetricsPort, MetricsPath, err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)

	if !strings.Contains(string(body), expectedMetrics[0]) {
		glog.Fatalf("Expected %s, but got %s", expectedMetrics[0], string(body))
	}

	glog.Infof("Serving Metrics on http://127.0.0.1%s%s.", MetricsPort, MetricsPath)
}

func mustGatherMetrics() {
	metrics, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		glog.Fatalf("failed to gather metrics: %s", err)
	}

	var diff []string
	for _, e := range expectedMetrics {
		found := false
		for _, m := range metrics {
			if e == *m.Name {
				found = true
				glog.Infof("Serving metric: %s", e)
				break
			}
		}
		if !found {
			diff = append(diff, e)
		}
	}

	if len(diff) != 0 {
		glog.Fatalf("Missing metrics\n%q", diff)
	}
}
