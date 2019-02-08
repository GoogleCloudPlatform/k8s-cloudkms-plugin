// Package pluginclient contains logic for interacting with K8S KMS Plugin.
package kmspluginclient

import (
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	"google.golang.org/grpc"

	"github.com/GoogleCloudPlatform/k8s-cloudkms-plugin/plugin"
)

// Client interacts with KMS Plugin via gRPC.
type Client struct {
	plugin.KeyManagementServiceClient
	connection *grpc.ClientConn
}

// Close closes the underlying gRPC connection to KMS Plugin.
func (k *Client) Close() {
	k.connection.Close()
}

// New constructs Client.
func New(endpoint string) (*Client, error) {
	addr, err := parseEndpoint(endpoint)
	if err != nil {
		return nil, err
	}

	connection, err := grpc.Dial(addr, grpc.WithInsecure(), grpc.WithDefaultCallOptions(grpc.FailFast(false)), grpc.WithDialer(
		func(string, time.Duration) (net.Conn, error) {
			// Ignoring addr and timeout arguments:
			// addr - comes from the closure
			// timeout - is ignored since we are connecting in a non-blocking configuration
			c, err := net.DialTimeout("unix", addr, 0)
			if err != nil {
				return nil, fmt.Errorf("failed to create connection to unix socket: %s, error: %v", addr, err)
			}
			return c, nil
		}))

	if err != nil {
		return nil, fmt.Errorf("failed to create connection to %s, error: %v", addr, err)
	}

	kmsClient := plugin.NewKeyManagementServiceClient(connection)
	return &Client{
		KeyManagementServiceClient: kmsClient,
		connection:                 connection,
	}, nil
}

// Parse the endpoint to extract schema, host or path.
func parseEndpoint(endpoint string) (string, error) {
	if len(endpoint) == 0 {
		return "", fmt.Errorf("remote KMS provider can't use empty string as endpoint")
	}

	u, err := url.Parse(endpoint)
	if err != nil {
		return "", fmt.Errorf("invalid endpoint %q for remote KMS provider, error: %v", endpoint, err)
	}

	if u.Scheme != "unix" {
		return "", fmt.Errorf("unsupported scheme %q for remote KMS provider", u.Scheme)
	}

	// Linux abstract namespace socket - no physical file required
	// Warning: Linux Abstract sockets have not concept of ACL (unlike traditional file based sockets).
	// However, Linux Abstract sockets are subject to Linux networking namespace, so will only be accessible to
	// containers within the same pod (unless host networking is used).
	if strings.HasPrefix(u.Path, "/@") {
		return strings.TrimPrefix(u.Path, "/"), nil
	}

	return u.Path, nil
}
