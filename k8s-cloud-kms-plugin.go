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
)

var (
	metricsPort = flag.String("metrics-addr", ":8080", "Address at which to publish metrics")
	metricsPath = flag.String("metrics-path", "/metrics", "Path at which to publish metrics")

	projectID  = flag.String("project-id", "", "Cloud project where KMS key-ring is hosted")
	locationID = flag.String("location-id", "global", "Location of the key-ring")
	keyRingID  = flag.String("key-ring-id", "", "ID of the key-ring where keys are stored")
	keyID      = flag.String("key-id", "", "Id of the key use for crypto operations")

	pathToUnixSocket = flag.String("path-to-unix-socket", "/tmp/kms-plugin.socket", "Full path to Unix socket that is used for communicating with KubeAPI Server")
)

func main() {
	flag.Parse()

	glog.Infof("Starting cloud KMS Plugin.")

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
		glog.Fatalf("failed to start gRPC Server, %v", err)
	}

	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("ok"))

	})
	go glog.Fatal(http.ListenAndServe(":8081", nil))

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
