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
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"regexp"

	"github.com/golang/glog"
	"github.com/immutablet/k8s-cloudkms-plugin/plugin"
)

var (
	metricsPort = flag.String("metrics-addr", plugin.MetricsPort, "Address at which to publish metrics")
	metricsPath = flag.String("metrics-path", plugin.MetricsPath, "Path at which to publish metrics")

	healthzPort = flag.String("healthz-addr", plugin.HealthzPort, "Address at which to publish healthz")
	healthzPath = flag.String("healthz-path", plugin.HealthzPath, "Path at which to publish healthz")

	gceConf          = flag.String("gce-config", "", "Path to gce.conf, if running on GKE.")
	keyURI           = flag.String("key-uri", "", "Uri of the key use for crypto operations (ex. projects/my-project/locations/my-location/keyRings/my-key-ring/cryptoKeys/my-key)")
	pathToUnixSocket = flag.String("path-to-unix-socket", plugin.PathToUnixSocket, "Full path to Unix socket that is used for communicating with KubeAPI Server")
)

func main() {
	mustValidateFlags()

	kmsPlugin, err := plugin.New(*keyURI, *pathToUnixSocket, *gceConf)
	if err != nil {
		glog.Fatalf("failed to instantiate kmsPlugin, %v", err)
	}

	kmsPlugin.MustServeKMSRequests(*healthzPath, *healthzPort, *metricsPath, *metricsPort)

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	sig := <-signals

	glog.Infof("Captured %v", sig)
	glog.Infof("Shutting down server")
	kmsPlugin.GracefulStop()
	glog.Infof("Exiting...")
	os.Exit(0)
}

func mustValidateFlags() {
	flag.Parse()

	keyURIPattern, err := regexp.Compile(plugin.KeyURIPattern)
	if err != nil {
		glog.Fatalf("Failed to compile keyURI regexp patter %s", keyURIPattern)
	}

	matched := keyURIPattern.MatchString(*keyURI)
	if !matched {
		glog.Fatalf("Supplied key-uri flag failed to match the expected regex pattern of %s", plugin.KeyURIPattern)
	}

	socketDir := filepath.Dir(*pathToUnixSocket)
	_, err = os.Stat(socketDir)
	glog.Infof("Unix Socket directory is %s", socketDir)
	if err != nil && os.IsNotExist(err) {
		glog.Fatalf(" Directory %s portion of path-to-unix-socket flag:%s does not exist.", socketDir, *pathToUnixSocket)
	}
	glog.Infof("Communication between KUBE API and KMS Plugin contaniners will be via %s", *pathToUnixSocket)

	if *gceConf != "" {
		_, err = os.Stat(*gceConf)
		if err != nil && os.IsNotExist(err) {
			glog.Fatalf("GCE Conf: %s does not exist.", *gceConf)
		}
		glog.Infof("Using GCE Config: %s.", *gceConf)
	}
}
