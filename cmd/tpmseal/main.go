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

	if err := ioutil.WriteFile(*publicAreaOutput, publicArea, 0644); err != nil {
		glog.Fatal(err)
	}
}
