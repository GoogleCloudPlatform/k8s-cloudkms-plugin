// Copyright 2024 Google LLC
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

package plugin

import (
	"strings"
	"testing"
)

const gceConfOnGKE = `[global]
token-url = https://my-project.googleapis.com/v1/masterProjects/722785932522/locations/us-central1-c/tokens
token-body = "{\"projectNumber\":722785932522,\"clusterId\":\"sandbox\"}"
project-id = iam-gke-custom-roles-master
network-name = default
subnetwork-name = default
node-instance-prefix = gke-sandbox
node-tags = gke-sandbox-ac7f9ea0-node`

const gceConfOnGCE = `[global]
project-id = alextc-k8s-lab
network-project-id = alextc-k8s-lab
network-name = default
subnetwork-name = default
node-instance-prefix = kubernetes-minion
node-tags = kubernetes-minion`

func TestExtractTokenConfigOnHostedMaster(t *testing.T) {
	t.Parallel()

	r := strings.NewReader(gceConfOnGKE)
	c, err := readConfig(r)
	if c == nil || err != nil {
		t.Fatalf("Failed to read gce.conf, err: %s", err)
	}

	if c.Global.TokenURL != "https://my-project.googleapis.com/v1/masterProjects/722785932522/locations/us-central1-c/tokens" {
		t.Fatalf("Failed to extract TokenURL")
	}

	if c.Global.TokenBody != "{\"projectNumber\":722785932522,\"clusterId\":\"sandbox\"}" {
		t.Fatalf("Failed to extract TokenBody")
	}
}

func TestShouldReturnNilOnGCEMaster(t *testing.T) {
	t.Parallel()

	r := strings.NewReader(gceConfOnGCE)
	c, err := readConfig(r)
	if c == nil {
		t.Fatalf("Failed to read gce.conf, err: %s", err)
	}

	if (tokenConfig{}) != *c {
		t.Fatalf("Alt Token Info should be empty on GCE.")
	}
}
