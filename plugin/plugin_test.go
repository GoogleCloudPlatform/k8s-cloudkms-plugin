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
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/GoogleCloudPlatform/k8s-cloudkms-plugin/tests"
	k8spb "github.com/GoogleCloudPlatform/k8s-cloudkms-plugin/v1beta1"
	"github.com/prometheus/client_golang/prometheus"

	"golang.org/x/net/context"
)

// Logger allows t.Testing and b.Testing to be passed to a method that executes testing logic.
type Logger interface {
	Errorf(format string, args ...interface{})
	Fatalf(format string, args ...interface{})
	Fatal(args ...interface{})
	Logf(format string, args ...interface{})
}

func TestE2E(t *testing.T) {
	p, err := New(tests.TestKeyURI, getSocketAddress(), "")
	if err != nil {
		t.Fatalf("failed to instantiate plugin, %v", err)
	}

	sut := NewOrchestrator(p, HealthzPath, HealthzPort, MetricsPath, MetricsPort)
	sut.Run()

	time.Sleep(1 * time.Millisecond)

	mustGetHTTPBody(t, HealthzPort, HealthzPath, "ok")
	mustGetHTTPBody(t, MetricsPort, MetricsPath, tests.MetricsOfInterest[0])

	mustGatherMetrics(t)
	printMetrics(t)
}

func TestEncryptDecrypt(t *testing.T) {
	plainText := []byte("secret")

	sut, err := New(tests.TestKeyURI, getSocketAddress(), "")
	if err != nil {
		t.Fatalf("failed to instantiate plugin, %v", err)
	}

	encryptRequest := k8spb.EncryptRequest{Version: apiVersion, Plain: []byte(plainText)}
	encryptResponse, err := sut.Encrypt(context.Background(), &encryptRequest)

	if err != nil {
		t.Fatal(err)
	}

	decryptRequest := k8spb.DecryptRequest{Version: apiVersion, Cipher: []byte(encryptResponse.Cipher)}
	decryptResponse, err := sut.Decrypt(context.Background(), &decryptRequest)
	if err != nil {
		t.Error(err)
	}

	if string(decryptResponse.Plain) != string(plainText) {
		t.Fatalf("Expected secret, but got %s", string(decryptResponse.Plain))
	}
}

func TestDecryptionError(t *testing.T) {
	plainText := []byte("secret")

	sut, err := New(tests.TestKeyURI, getSocketAddress(), "")
	if err != nil {
		t.Fatalf("failed to instantiate plugin, %v", err)
	}

	encryptRequest := k8spb.EncryptRequest{Version: apiVersion, Plain: []byte(plainText)}
	encryptResponse, err := sut.Encrypt(context.Background(), &encryptRequest)

	if err != nil {
		t.Fatal(err)
	}

	decryptRequest := k8spb.DecryptRequest{Version: apiVersion, Cipher: []byte(encryptResponse.Cipher[1:])}
	_, err = sut.Decrypt(context.Background(), &decryptRequest)
	if err == nil {
		t.Fatal(err)
	}
}

func TestRPC(t *testing.T) {
	plugin, client, err := setup()
	defer plugin.Stop()
	if err != nil {
		t.Fatalf("setup failed, %v", err)
	}

	runGRPCTest(t, client, []byte("secret"))
}

func BenchmarkRPC(b *testing.B) {
	b.StopTimer()
	plugin, client, err := setup()
	defer plugin.Stop()
	if err != nil {
		b.Fatalf("setup failed, %v", err)
	}

	b.StartTimer()
	for i := 0; i < b.N; i++ {
		runGRPCTest(b, client, []byte("secret"+strconv.Itoa(i)))
	}
	b.StopTimer()
	printMetrics(b)
}

func setup() (*Plugin, k8spb.KeyManagementServiceClient, error) {
	sut, err := New(tests.TestKeyURI, getSocketAddress(), "")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to instantiate plugin, %v", err)
	}
	err = sut.setupRPCServer()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to start gRPC Server, %v", err)
	}

	go sut.Server.Serve(sut.Listener)

	connection, err := sut.newUnixSocketConnection()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open unix socket, %v", err)
	}

	client := k8spb.NewKeyManagementServiceClient(connection)
	return sut, client, nil
}

func runGRPCTest(l Logger, client k8spb.KeyManagementServiceClient, plainText []byte) {
	encryptRequest := k8spb.EncryptRequest{Version: apiVersion, Plain: plainText}
	encryptResponse, err := client.Encrypt(context.Background(), &encryptRequest)
	if err != nil {
		l.Fatal(err)
	}

	decryptRequest := k8spb.DecryptRequest{Version: apiVersion, Cipher: []byte(encryptResponse.Cipher)}
	decryptResponse, err := client.Decrypt(context.Background(), &decryptRequest)
	if err != nil {
		l.Fatal(err)
	}

	if string(decryptResponse.Plain) != string(plainText) {
		l.Fatalf("Expected secret, but got %s", string(decryptResponse.Plain))
	}

	printMetrics(l)
}

func printMetrics(l Logger) error {
	metrics, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		return fmt.Errorf("failed to gather metrics: %s", err)
	}

	for _, mf := range metrics {
		// l.Logf("%s", *mf.Name)
		if contains(tests.MetricsOfInterest, *mf.Name) {
			for _, metric := range mf.GetMetric() {
				l.Logf("%v", metric)
			}
		}
	}

	return nil
}

func mustGatherMetrics(l Logger) {
	metrics, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		l.Fatalf("failed to gather metrics: %s", err)
	}

	expectedCount := len(tests.MetricsOfInterest)
	actualCount := 0

	for _, mf := range metrics {
		if contains(tests.MetricsOfInterest, *mf.Name) {
			actualCount++
		}
	}

	if expectedCount != actualCount {
		l.Fatalf("Expected %d metrics, but got %d", expectedCount, actualCount)
	}
}

func ExampleEncrypt() {
	plainText := []byte("secret")

	plugin, err := New(tests.TestKeyURI, getSocketAddress(), "")
	if err != nil {
		log.Fatalf("failed to instantiate plugin, %v", err)
	}

	encryptRequest := k8spb.EncryptRequest{Version: "v1beta1", Plain: []byte(plainText)}
	encryptResponse, err := plugin.Encrypt(context.Background(), &encryptRequest)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Cipher: %s", string(encryptResponse.Cipher))
}

func ExampleDecrypt() {
	cipher := "secret goes here"

	plugin, err := New(tests.TestKeyURI, getSocketAddress(), "")
	if err != nil {
		log.Fatalf("failed to instantiate plugin, %v", err)
	}

	decryptRequest := k8spb.DecryptRequest{Version: "v1beta1", Cipher: []byte(cipher)}
	decryptResponse, err := plugin.Decrypt(context.Background(), &decryptRequest)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Plain: %s", string(decryptResponse.Plain))
}

func mustGetHTTPBody(l Logger, port, path, expect string) {
	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1%s%s", port, path))
	if err != nil {
		l.Fatalf("Failed to reach %s%s: %v", port, path, err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)

	if !strings.Contains(string(body), expect) {
		l.Fatalf("Expected %s, but got %s", expect, string(body))
	}
}

func getSocketAddress() string {
	return fmt.Sprintf("@%d", rand.Intn(100000))
}
