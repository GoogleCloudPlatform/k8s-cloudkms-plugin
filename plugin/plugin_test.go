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

import(
	"log"
	"testing"

	k8spb "github.com/immutablet/k8s-kms-plugin/v1beta1"

	"golang.org/x/net/context"
	"fmt"
	"net"
	"time"
	"google.golang.org/grpc"
)

const (
	projectID string = "cloud-kms-lab"
	locationID string = "us-central1"
	keyRingID string = "ring-01"
	keyID string = "my-key"
	pathToUnixSocket = "/tmp/test.socket"
)

func TestEncryptDecrypt(t *testing.T) {
	plainText := []byte("secret")

	sut := New(projectID, locationID, keyRingID, keyID, pathToUnixSocket)

	encryptRequest := k8spb.EncryptRequest{Version: version, Plain: []byte(plainText)}
	encryptResponse, err := sut.Encrypt(context.Background(), &encryptRequest)

	if err != nil {
		t.Error(err)
	}

	decryptRequest := k8spb.DecryptRequest{Version: version, Cipher: []byte(encryptResponse.Cipher)}
	decryptResponse, err := sut.Decrypt(context.Background(), &decryptRequest)
	if err != nil {
		t.Error(err)
	}

	if string(decryptResponse.Plain) != string(plainText) {
		t.Errorf("Expected secret, but got %s", string(decryptResponse.Plain))
	}
}

func TestDecryptionError(t *testing.T) {

	plainText := []byte("secret")

	sut := New(projectID, locationID, keyRingID, keyID, pathToUnixSocket)

	encryptRequest := k8spb.EncryptRequest{Version: version, Plain: []byte(plainText)}
	encryptResponse, err := sut.Encrypt(context.Background(), &encryptRequest)

	if err != nil {
		t.Error(err)
	}

	decryptRequest := k8spb.DecryptRequest{Version: version, Cipher: []byte(encryptResponse.Cipher[1:])}
	_, err = sut.Decrypt(context.Background(), &decryptRequest)
	if err == nil {
		t.Error(err)
	}

}

func TestRPC(t *testing.T) {
	plainText := []byte("secret")
	sut := New(projectID, locationID, keyRingID, keyID, pathToUnixSocket)
	err := sut.Start()
	if err != nil {
		t.Error(err)
	}
	defer sut.Stop()

	connection, err := newUnixSocketConnection(pathToUnixSocket)
	if err != nil {
		t.Error(err)
	}
	defer connection.Close()

	client := k8spb.NewKMSServiceClient(connection)

	encryptRequest := k8spb.EncryptRequest{Version: version, Plain: plainText}
	encryptResponse, err := client.Encrypt(context.Background(), &encryptRequest)
	if err != nil {
		t.Error(err)
	}

	decryptRequest := k8spb.DecryptRequest{Version: version, Cipher: []byte(encryptResponse.Cipher)}
	decryptResponse, err := client.Decrypt(context.Background(), &decryptRequest)
	if err != nil {
		t.Error(err)
	}

	if string(decryptResponse.Plain) != string(plainText) {
		t.Errorf("Expected secret, but got %s", string(decryptResponse.Plain))
	}
}

func newUnixSocketConnection(path string) (*grpc.ClientConn, error)  {
	protocol, addr := "unix", path
	dialer := func(addr string, timeout time.Duration) (net.Conn, error) {
		return net.DialTimeout(protocol, addr, timeout)
	}
	connection, err := grpc.Dial(addr, grpc.WithInsecure(), grpc.WithDialer(dialer))
	if err != nil {
		return nil, err
	}

	return connection, nil
}

func ExampleEncrypt() {
	plainText := []byte("secret")

	plugin := New(projectID, locationID, keyRingID, keyID, pathToUnixSocket)

	encryptRequest := k8spb.EncryptRequest{Version: "v1beta1", Plain: []byte(plainText)}
	encryptResponse, err := plugin.Encrypt(context.Background(), &encryptRequest)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Cipher: %s", string(encryptResponse.Cipher))
}

func ExampleDecrypt() {
	cipher := "secret goes here"

	plugin := New(projectID, locationID, keyRingID, keyID, pathToUnixSocket)

	decryptRequest := k8spb.DecryptRequest{Version: "v1beta1", Cipher: []byte(cipher)}
	decryptResponse, err := plugin.Decrypt(context.Background(), &decryptRequest)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Plain: %s", string(decryptResponse.Plain))
}
