package plugin

import (
	"strings"
	"testing"
)

const gceConf = `[global]
token-url = https://my-project.googleapis.com/v1/masterProjects/722785932522/locations/us-central1-c/tokens
token-body = "{\"projectNumber\":722785932522,\"clusterId\":\"sandbox\"}"
project-id = iam-gke-custom-roles-master
network-name = default
subnetwork-name = default
node-instance-prefix = gke-sandbox
node-tags = gke-sandbox-ac7f9ea0-node`

func TestExtractTokenConfig(t *testing.T) {
	r := strings.NewReader(gceConf)
	c, err := readConfig(r)
	if c == nil {
		t.Fatalf("Failed to read gce.conf, err: %s", err)
	}

	if c.Global.TokenURL != "https://my-project.googleapis.com/v1/masterProjects/722785932522/locations/us-central1-c/tokens" {
		t.Fatalf("Failed to extract TokenURL")
	}

	if c.Global.TokenBody != "{\"projectNumber\":722785932522,\"clusterId\":\"sandbox\"}" {
		t.Fatalf("Failed to extract TokenBody")
	}
}
