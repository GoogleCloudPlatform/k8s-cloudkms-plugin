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
	out               = flag.String("path-to-output", "", "Path to output.")
	pathToPrivateArea = flag.String("path-to-priv-area", "priv.bin", "Path to the private area.")
	pathToPublicArea  = flag.String("path-to-pub-area", "pub.bin", "Path to the public area.")
)

func main() {
	flag.Parse()

	privateArea, err := ioutil.ReadFile(*pathToPrivateArea)
	if err != nil {
		glog.Fatal(err)
	}

	publicArea, err := ioutil.ReadFile(*pathToPublicArea)
	if err != nil {
		glog.Fatal(err)
	}

	c, err := tpm.Unseal(*pathToTPM, *pcrToMeasure, "", "", privateArea, publicArea)
	if err != nil {
		glog.Fatal(err)
	}

	if err := ioutil.WriteFile(*out, c, 0644); err != nil {
		glog.Fatal(err)
	}
}
