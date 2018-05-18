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
	r := strings.NewReader(gceConfOnGCE)
	c, err := readConfig(r)
	if c == nil {
		t.Fatalf("Failed to read gce.conf, err: %s", err)
	}

	if (tokenConfig{}) != *c {
		t.Fatalf("Alt Token Info should be empty on GCE.")
	}
}
