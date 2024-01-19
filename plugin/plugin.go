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

// Package plugin implements CloudKMS plugin for GKE as described in go/gke-secrets-encryption-design.
package plugin

import (
	"net"

	"google.golang.org/api/cloudkms/v1"
	"google.golang.org/grpc"
)

type PluginConfig struct {
	KeyService       *cloudkms.ProjectsLocationsKeyRingsCryptoKeysService
	KeyURI           string
	KeySuffix        string
	PathToUnixSocket string
	// Embedding these only to shorten access to fields.
	net.Listener
	*grpc.Server
}

// Plugin is a CloudKMS plugin for K8S.
type Plugin interface {
	// Starts gRPC server or dies.
	ServeKMSRequests() (*grpc.Server, chan error)
}
