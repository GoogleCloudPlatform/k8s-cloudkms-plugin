// Copyright 2019 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"flag"
	"io/ioutil"

	"github.com/GoogleCloudPlatform/k8s-cloudkms-plugin/tpm"
	"github.com/golang/glog"
)

var (
	pathToTPM         = flag.String("path-to-tpm", "/dev/tpmrm0", "Path to tpm device or tpm resource manager.")
	pcrToMeasure      = flag.Int("pcr-to-measure", 7, "PCR to measure.")
	pathToPlaintext   = flag.String("path-to-plaintext", "", "Data to seal.")
	privateAreaOutput = flag.String("path-to-priv-area", "priv.bin", "Path to where to place the private area.")
	publicAreaOutput  = flag.String("path-to-pub-area", "pub.bin", "Path to where to place the public area.")
)

func main() {
	flag.Parse()

	d, err := ioutil.ReadFile(*pathToPlaintext)
	if err != nil {
		glog.Fatal(err)
	}

	privateArea, publicArea, err := tpm.Seal(*pathToTPM, *pcrToMeasure, "", "", d)
	if err != nil {
		glog.Fatal(err)
	}

	if err := ioutil.WriteFile(*privateAreaOutput, privateArea, 0644); err != nil {
		glog.Fatal(err)
	}

	if err := ioutil.WriteFile(*publicAreaOutput, publicArea, 0600); err != nil {
		glog.Fatal(err)
	}
}
