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

// Command fakekms simulates CloudKMS service - use only in integration tests.
package main

import (
	"flag"
	"os"
	"os/signal"
	"syscall"

	"github.com/golang/glog"

	"github.com/GoogleCloudPlatform/k8s-cloudkms-plugin/testutils/fakekms"
)

var (
	port    = flag.Int("port", 8085, "Port on which to listen.")
	keyName = flag.String("key-name", "", "Name of CloudKMS key.")
)

func main() {

	if *keyName == "" {
		glog.Exit("key-name is a mandatory argument.")
	}

	s, err := fakekms.NewWithPipethrough(*keyName, *port)
	if err != nil {
		glog.Exitf("failed to start FakeKMS, error %v", err)
	}
	defer s.Close()
	glog.Infof("FakeKMS is listening on port: %d", *port)

	signalsChan := make(chan os.Signal, 1)
	signal.Notify(signalsChan, syscall.SIGINT, syscall.SIGTERM)

	sig := <-signalsChan
	glog.Exitf("captured %v, shutting down fakeKMS", sig)
}
