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
	"strings"
)

var (
	metricsPort = flag.String("metrics-addr", plugin.MetricsPort, "Address at which to publish metrics")
	metricsPath = flag.String("metrics-path", plugin.MetricsPath, "Path at which to publish metrics")

	healthzPort = flag.String("healthz-addr", plugin.HealthzPort, "Address at which to publish healthz")
	healthzPath = flag.String("healthz-path", plugin.HealthzPath, "Path at which to publish healthz")

	gceConf          = flag.String("gce-config", "", "Path to gce.conf, if running on GKE.")
	keyURI           = flag.String("key-uri", "", "Uri of the key use for crypto operations (ex. projects/my-project/locations/my-location/keyRings/my-key-ring/cryptoKeys/my-key)")
	pathToUnixSocket = flag.String("path-to-unix-socket", "@kms-plugin-socket", "Full path to Unix socket that is used for communicating with KubeAPI Server, or Linux socket namespace object - must start with @")
)

func main() {
	mustValidateFlags()

	p, err := plugin.New(*keyURI, *pathToUnixSocket, *gceConf)
	if err != nil {
		glog.Fatalf("failed to instantiate kmsPlugin, %v", err)
	}

	o := plugin.NewOrchestrator(p, *healthzPath, *healthzPort, *metricsPath, *metricsPort)
	o.Run()

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	sig := <-signals

	glog.Infof("Captured %v", sig)
	glog.Infof("Shutting down server")
	p.GracefulStop()
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
		glog.Fatalf("Supplied key-uri flag: %s failed to match the expected regex pattern of %s", *keyURI, plugin.KeyURIPattern)
	}

	// Using an actual socket file instead of in-memory Linux socket namespace object.
	glog.Infof("Checking socket path %s", *pathToUnixSocket)
	if ! strings.HasPrefix(*pathToUnixSocket, "@") {
		socketDir := filepath.Dir(*pathToUnixSocket)
		_, err = os.Stat(socketDir)
		glog.Infof("Unix Socket directory is %s", socketDir)
		if err != nil && os.IsNotExist(err) {
			glog.Fatalf(" Directory %s portion of path-to-unix-socket flag:%s does not exist.", socketDir, *pathToUnixSocket)
		}
	}
	glog.Infof("Communication between KUBE API and KMS Plugin containers will be via %s", *pathToUnixSocket)

	if *gceConf != "" {
		_, err = os.Stat(*gceConf)
		if err != nil && os.IsNotExist(err) {
			glog.Fatalf("GCE Conf: %s does not exist.", *gceConf)
		}
		glog.Infof("Using GCE Config: %s.", *gceConf)
	}
}
