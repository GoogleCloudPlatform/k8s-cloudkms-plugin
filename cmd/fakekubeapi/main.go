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

// Command fake-kube-apiserver simulates k8s kube-apiserver - use only in integration tests.
package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/golang/glog"

	"github.com/GoogleCloudPlatform/k8s-cloudkms-plugin/testutils/fakekubeapi"
	"github.com/GoogleCloudPlatform/k8s-cloudkms-plugin/testutils/kmspluginclient"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	port          = flag.Int("port", 8080, "Port on which to listen.")
	timeout       = flag.Duration("kms-plugin-timeout", 3*time.Second, "Timeout for calls to kms-plugin in seconds.")
	kmsSocketPath = flag.String("path-to-kms-socket", "", "Path to Unix Domain socket for communicating with KMS Plugin.")

	// TODO(immutableT) Supply option to consume resources from yaml files.
	namespaces = corev1.NamespaceList{
		Items: []corev1.Namespace{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "default",
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "system",
				},
			},
		},
	}

	secrets = map[string][]corev1.Secret{
		"default": {
			{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Secret",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-default-secret",
					Namespace: "default",
				},
				Data: map[string][]byte{"token": []byte("fakeTokenDefault")},
			},
		},
		"system": {
			{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Secret",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-system-secret",
					Namespace: "system",
				},
				Data: map[string][]byte{"token": []byte("fakeTokenSystem")},
			},
		},
	}
)

func main() {

	if *kmsSocketPath == "" {
		glog.Exitln("path-to-kms-socket is mandatory argument")
	}

	k, err := kmspluginclient.New(fmt.Sprintf("unix://%s", *kmsSocketPath))
	if err != nil {
		glog.Exitf("Failed to initialize KMS Client, error: %v", err)
	}

	s, err := fakekubeapi.New(namespaces, secrets, *port, k, *timeout)
	if err != nil {
		glog.Exitf("failed to start fake kube-apiserver, error %v", err)
	}
	defer s.Close()
	glog.Infof("kube-apiserver %v is listening on port: %d", s.URL(), *port)

	signalsChan := make(chan os.Signal, 1)
	signal.Notify(signalsChan, syscall.SIGINT, syscall.SIGTERM)

	sig := <-signalsChan
	glog.Exitf("captured %v, shutting down fake kube-apiserver", sig)
}
