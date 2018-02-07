package main

import (
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/immutablet/k8s-kms-plugin/plugin"
	"flag"
	"net/http"
	"github.com/golang/glog"
)

var (
	metricsPort = flag.String("metrics-addr", ":8080", "Address at which to publish metrics")
	metricsPath = flag.String("metrics-path", "/metrics", "Path at which to publish metrics")

	projectID = flag.String("project-id", "", "Cloud project where KMS key-ring is hosted")
	locationID = flag.String("location-id", "global", "Location of the key-ring")
	keyRingID = flag.String("key-ring-id", "", "ID of the key-ring where keys are stored")
	keyID = flag.String("key-id", "", "Id of the key use for crypto operations")

	pathToUnixSocket = flag.String("path-to-unix-socket", "", "Full path to Unix socket that is used for communicating with KubeAPI Server")
)

func main() {
	flag.Parse()

	go func() {
		http.Handle(*metricsPath, promhttp.Handler())
		glog.Fatal(http.ListenAndServe(*metricsPort, nil))
	}()

	sut, err := plugin.New(*projectID, *locationID, *keyRingID, *keyID, *pathToUnixSocket)
	if err != nil {
		glog.Fatalf("failed to instantiate plugin, %v", err)
	}
	err = sut.Start()
	if err != nil {
		glog.Fatalf("failed to start gRPC Server, %v", err)
	}
}

