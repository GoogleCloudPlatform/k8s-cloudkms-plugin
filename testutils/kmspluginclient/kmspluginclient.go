// Copyright 2018 Google LLC
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

// Package pluginclient contains logic for interacting with K8S KMS Plugin.
package kmspluginclient

import (
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	plugin "github.com/GoogleCloudPlatform/k8s-cloudkms-plugin/plugin/v1"
	"google.golang.org/grpc"
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
