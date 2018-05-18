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
	"golang.org/x/net/context"
	cloudkms "google.golang.org/api/cloudkms/v1"

	"github.com/golang/glog"
	k8spb "github.com/GoogleCloudPlatform/k8s-cloudkms-plugin/v1beta1"
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

	encryptRequest := k8spb.EncryptRequest{Version: apiVersion, Plain: []byte(plainText)}
	encryptResponse, err := v.Encrypt(context.Background(), &encryptRequest)

	if err != nil {
		glog.Fatalf("failed to ping KMS: %v", err)
	}

	decryptRequest := k8spb.DecryptRequest{Version: apiVersion, Cipher: []byte(encryptResponse.Cipher)}
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
	client := k8spb.NewKeyManagementServiceClient(connection)

	plainText := []byte("secret")

	encryptRequest := k8spb.EncryptRequest{Version: apiVersion, Plain: []byte(plainText)}
	encryptResponse, err := client.Encrypt(context.Background(), &encryptRequest)

	if err != nil {
		glog.Fatalf("failed to ping KMS: %v", err)
	}

	decryptRequest := k8spb.DecryptRequest{Version: apiVersion, Cipher: []byte(encryptResponse.Cipher)}
	decryptResponse, err := client.Decrypt(context.Background(), &decryptRequest)
	if err != nil {
		glog.Fatalf("failed to ping KMS gRPC: %v", err)
	}

	if string(decryptResponse.Plain) != string(plainText) {
		glog.Fatalf("failed to ping KMS gRPC, expected secret, but got %s", string(decryptResponse.Plain))
	}

	glog.Infof("Successfully pinged gRPC KMS.")
}

func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}
