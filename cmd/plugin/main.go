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

// Binary kmsplugin - entry point into kms-plugin. See go/gke-secrets-encryption-design for details.
package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"google.golang.org/api/cloudkms/v1"

	"github.com/GoogleCloudPlatform/k8s-cloudkms-plugin/plugin"

	"github.com/golang/glog"
)

var (
	healthzPort    = flag.Int("healthz-port", 8081, "Port on which to publish healthz")
	healthzPath    = flag.String("healthz-path", "healthz", "Path at which to publish healthz")
	healthzTimeout = flag.Duration("healthz-timeout", 5*time.Second, "timeout in seconds for communicating with the unix socket")

	metricsPort             = flag.Int("metrics-port", 8082, "Port on which to publish metrics")
	metricsPath             = flag.String("metrics-path", "metrics", "Path at which to publish metrics")
	authTokenRequestTimeout = flag.Duration("token-request-timeout", 5*time.Second, "timeout in seconds for requesting auth token")

	gceConf          = flag.String("gce-config", "", "Path to gce.conf, if running on GKE.")
	keyURI           = flag.String("key-uri", "", "Uri of the key use for crypto operations (ex. projects/my-project/locations/my-location/keyRings/my-key-ring/cryptoKeys/my-key)")
	pathToUnixSocket = flag.String("path-to-unix-socket", "/var/run/kmsplugin/socket.sock", "Full path to Unix socket that is used for communicating with KubeAPI Server, or Linux socket namespace object - must start with @")

	// Integration testing arguments.
	integrationTest = flag.Bool("integration-test", false, "When set to true, http.DefaultClient will be used, as opposed callers identity acquired with a TokenService.")
	fakeKMSPort     = flag.Int("fake-kms-port", 8085, "Port for Fake KMS, only use in integration tests.")
)

func main() {
	mustValidateFlags()

	ctx, cancel := context.WithTimeout(context.Background(), *authTokenRequestTimeout)
	defer cancel()

	var (
		httpClient = http.DefaultClient
		err        error
	)

	if !*integrationTest {
		httpClient, err = plugin.NewHTTPClient(ctx, *gceConf)
		if err != nil {
			glog.Exitf("failed to instantiate http httpClient: %v", err)
		}
	}

	kms, err := cloudkms.New(httpClient)
	if err != nil {
		glog.Exitf("failed to instantiate cloud kms httpClient: %v", err)
	}

	if *integrationTest {
		kms.BasePath = fmt.Sprintf("http://localhost:%d", *fakeKMSPort)
	}

	healthz := &plugin.HealthZ{
		KeyName:        *keyURI,
		KeyService:     kms.Projects.Locations.KeyRings.CryptoKeys,
		UnixSocketPath: *pathToUnixSocket,
		CallTimeout:    *healthzTimeout,
		ServingURL: &url.URL{
			Host: net.JoinHostPort("localhost", strconv.FormatUint(uint64(*healthzPort), 10)),
			Path: *healthzPath,
		},
	}

	metrics := &plugin.Metrics{
		ServingURL: &url.URL{
			Host: net.JoinHostPort("localhost", strconv.FormatUint(uint64(*metricsPort), 10)),
			Path: *metricsPath,
		},
	}

	glog.Exit(run(plugin.New(kms.Projects.Locations.KeyRings.CryptoKeys, *keyURI, *pathToUnixSocket), healthz, metrics))
}

func run(p *plugin.Plugin, h *plugin.HealthZ, m *plugin.Metrics) error {
	signalsChan := make(chan os.Signal, 1)
	signal.Notify(signalsChan, syscall.SIGINT, syscall.SIGTERM)

	metricsErrChan := m.Serve()
	healthzErrChan := h.Serve()

	gRPCSrv, kmsErrorChan := p.ServeKMSRequests()
	defer gRPCSrv.GracefulStop()

	for {
		select {
		case sig := <-signalsChan:
			return fmt.Errorf("captured %v, shutting down kms-plugin", sig)
		case kmsError := <-kmsErrorChan:
			return kmsError
		case metricsErr := <-metricsErrChan:
			// Limiting this to warning only - will run without metrics.
			glog.Warning(metricsErr)
			metricsErrChan = nil
		case healthzErr := <-healthzErrChan:
			// Limiting this to warning only - will run without healthz.
			glog.Warning(healthzErr)
			healthzErrChan = nil
		}
	}
}

func mustValidateFlags() {
	glog.Infof("Checking socket path %q", *pathToUnixSocket)
	socketDir := filepath.Dir(*pathToUnixSocket)
	glog.Infof("Unix Socket directory is %q", socketDir)
	if _, err := os.Stat(socketDir); err != nil {
		glog.Exitf(" Directory %q portion of path-to-unix-socket flag:%q does not seem to exist.", socketDir, *pathToUnixSocket)
	}
	glog.Infof("Communication between KUBE API and KMS Plugin containers will be via %q", *pathToUnixSocket)
}
