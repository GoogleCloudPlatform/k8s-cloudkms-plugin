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

// Package plugin implements CloudKMS plugin for GKE as described in go/gke-secrets-encryption-design.
package plugin

import (
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/golang/glog"
	"google.golang.org/api/cloudkms/v1"
	"google.golang.org/grpc"
)

const (
	netProtocol = "unix"
)

type AbstractPlugin struct {
	Plugin
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
	RegisterKeyManagementServiceServer()
}

// ServeKMSRequests starts gRPC server or dies.
func (g *AbstractPlugin) ServeKMSRequests() (*grpc.Server, chan error) {
	errorChan := make(chan error, 1)
	if err := g.setupRPCServer(); err != nil {
		errorChan <- err
		close(errorChan)
		return nil, errorChan
	}

	go func() {
		defer close(errorChan)
		errorChan <- g.Serve(g.Listener)
	}()

	return g.Server, errorChan
}

func (g *AbstractPlugin) setupRPCServer() error {
	if err := g.cleanSockFile(); err != nil {
		return err
	}

	listener, err := net.Listen(netProtocol, g.PathToUnixSocket)
	if err != nil {
		return fmt.Errorf("failed to start listener, error: %v", err)
	}
	g.Listener = listener
	glog.Infof("Listening on unix domain socket: %s", g.PathToUnixSocket)

	g.Server = grpc.NewServer()
	g.RegisterKeyManagementServiceServer()

	return nil
}

func (g *AbstractPlugin) cleanSockFile() error {
	// @ implies the use of Linux socket namespace - no file on disk and nothing to clean-up.
	if strings.HasPrefix(g.PathToUnixSocket, "@") {
		return nil
	}

	err := os.Remove(g.PathToUnixSocket)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete the socket file, error: %v", err)
	}
	return nil
}
