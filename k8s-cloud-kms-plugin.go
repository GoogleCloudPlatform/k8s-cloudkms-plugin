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

package main

import (
	"flag"
	"path/filepath"
	"github.com/golang/glog"
	"github.com/immutablet/k8s-kms-plugin/plugin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"net/http"
	"os"
	k8spb "github.com/immutablet/k8s-kms-plugin/v1beta1"
	"golang.org/x/net/context"
	"time"
)

var (
	metricsPort = flag.String("metrics-addr", ":8081", "Address at which to publish metrics")
	metricsPath = flag.String("metrics-path", "/metrics", "Path at which to publish metrics")

	healthzPort = flag.String("healthz-addr", ":8082", "Address at which to publish healthz")
	healthzPath = flag.String("healthz-path", "/healthz", "Path at which to publish healthz")


	projectID  = flag.String("project-id", "", "Cloud project where KMS key-ring is hosted")
	locationID = flag.String("location-id", "global", "Location of the key-ring")
	keyRingID  = flag.String("key-ring-id", "", "ID of the key-ring where keys are stored")
	keyID      = flag.String("key-id", "", "Id of the key use for crypto operations")

	pathToUnixSocket = flag.String("path-to-unix-socket", "/tmp/kms-plugin.socket", "Full path to Unix socket that is used for communicating with KubeAPI Server")
)

func main() {
	flag.Parse()

	glog.Infof("Starting cloud KMS gRPC Plugin.")

	socketDir := filepath.Dir(*pathToUnixSocket)
	_, err := os.Stat(socketDir)
	glog.Infof("Unix Socket directory is %s", socketDir)
	if err != nil &&  os.IsNotExist(err) {
		glog.Fatalf(" Directory %s portion of path-to-unix-socket flag:%s does not exist.", socketDir, *pathToUnixSocket)
	}
	glog.Infof("Communicating with KUBE API via %s", *pathToUnixSocket)

	go func() {
		http.Handle(*metricsPath, promhttp.Handler())
		glog.Fatal(http.ListenAndServe(*metricsPort, nil))
	}()

	kmsPlugin, err := plugin.New(*projectID, *locationID, *keyRingID, *keyID, *pathToUnixSocket)
	if err != nil {
		glog.Fatalf("failed to instantiate kmsPlugin, %v", err)
	}
	mustPingKMS(kmsPlugin)

	err = kmsPlugin.SetupRPCServer()
	if err != nil {
		glog.Fatalf("failed to setup gRPC Server, %v", err)
	}

	glog.Infof("Pinging KMS gRPC in 10ms.")
	go func () {
		time.Sleep(10 * time.Millisecond)
		mustPingRPC(kmsPlugin)

		// Now we can declare healthz OK.
		http.HandleFunc(*healthzPath, handleHealthz)
		glog.Fatal(http.ListenAndServe(*healthzPort, nil))
	}()

	glog.Infof("About to server gRPC")

	err = kmsPlugin.Serve(kmsPlugin.Listener)
	if err != nil {
		glog.Fatalf("failed to serve gRPC, %v", err)
	}
}


func mustPingKMS(kms *plugin.Plugin) {
	plainText := []byte("secret")

	glog.Infof("Pinging KMS.")

	encryptRequest := k8spb.EncryptRequest{Version: plugin.APIVersion, Plain: []byte(plainText)}
	encryptResponse, err := kms.Encrypt(context.Background(), &encryptRequest)

	if err != nil {
		glog.Fatalf("failed to ping KMS: %v", err)
	}

	decryptRequest := k8spb.DecryptRequest{Version: plugin.APIVersion, Cipher: []byte(encryptResponse.Cipher)}
	decryptResponse, err := kms.Decrypt(context.Background(), &decryptRequest)
	if err != nil {
		glog.Fatalf("failed to ping KMS: %v", err)
	}

	if string(decryptResponse.Plain) != string(plainText) {
		glog.Fatalf("failed to ping kms, expected secret, but got %s", string(decryptResponse.Plain))
	}

	glog.Infof("Successfully pinged KMS.")
}

func mustPingRPC(kms *plugin.Plugin) {
	glog.Infof("Pinging KMS gRPC.")

	connection, err := kms.NewUnixSocketConnection()
	if err != nil {
		glog.Fatalf("failed to open unix socket, %v", err)
	}
	client := k8spb.NewKMSServiceClient(connection)

	plainText := []byte("secret")

	encryptRequest := k8spb.EncryptRequest{Version: plugin.APIVersion, Plain: []byte(plainText)}
	encryptResponse, err := client.Encrypt(context.Background(), &encryptRequest)

	if err != nil {
		glog.Fatalf("failed to ping KMS: %v", err)
	}

	decryptRequest := k8spb.DecryptRequest{Version: plugin.APIVersion, Cipher: []byte(encryptResponse.Cipher)}
	decryptResponse, err := client.Decrypt(context.Background(), &decryptRequest)
	if err != nil {
		glog.Fatalf("failed to ping KMS gRPC: %v", err)
	}

	if string(decryptResponse.Plain) != string(plainText) {
		glog.Fatalf("failed to ping KMS gRPC, expected secret, but got %s", string(decryptResponse.Plain))
	}

	glog.Infof("Successfully pinged gRPC KMS.")
}

func handleHealthz(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(200)
	w.Write([]byte("ok"))
}
