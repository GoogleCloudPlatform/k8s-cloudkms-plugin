package plugin

import (
	"golang.org/x/net/context"
	cloudkms "google.golang.org/api/cloudkms/v1"

	"github.com/golang/glog"
	k8spb "github.com/immutablet/k8s-cloudkms-plugin/v1beta1"
)

type Validator struct {
	*Plugin
}

func NewValidator(plugin *Plugin) *Validator {
	return &Validator{plugin}
}

func (v *Validator) mustValidatePrerequisites() {
	v.mustHaveIAMPermissions()
	v.mustPingKMS()
}

func (v *Validator) mustHaveIAMPermissions() {
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

func (v *Validator) mustPingKMS() {
	plainText := []byte("secret")

	glog.Infof("Pinging KMS.")

	encryptRequest := k8spb.EncryptRequest{Version: APIVersion, Plain: []byte(plainText)}
	encryptResponse, err := v.Encrypt(context.Background(), &encryptRequest)

	if err != nil {
		glog.Fatalf("failed to ping KMS: %v", err)
	}

	decryptRequest := k8spb.DecryptRequest{Version: APIVersion, Cipher: []byte(encryptResponse.Cipher)}
	decryptResponse, err := v.Decrypt(context.Background(), &decryptRequest)
	if err != nil {
		glog.Fatalf("failed to ping KMS: %v", err)
	}

	if string(decryptResponse.Plain) != string(plainText) {
		glog.Fatalf("failed to ping kms, expected secret, but got %s", string(decryptResponse.Plain))
	}

	glog.Infof("Successfully pinged KMS.")
}

func (v *Validator) mustPingRPC() {
	glog.Infof("Pinging KMS gRPC.")

	connection, err := v.newUnixSocketConnection()
	if err != nil {
		glog.Fatalf("failed to open unix socket, %v", err)
	}
	client := k8spb.NewKeyManagementServiceClient(connection)

	plainText := []byte("secret")

	encryptRequest := k8spb.EncryptRequest{Version: APIVersion, Plain: []byte(plainText)}
	encryptResponse, err := client.Encrypt(context.Background(), &encryptRequest)

	if err != nil {
		glog.Fatalf("failed to ping KMS: %v", err)
	}

	decryptRequest := k8spb.DecryptRequest{Version: APIVersion, Cipher: []byte(encryptResponse.Cipher)}
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
