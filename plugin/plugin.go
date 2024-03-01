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
	"google.golang.org/grpc"
)

const (
	netProtocol = "unix"
)

// Plugin is a CloudKMS plugin for K8S.
type Plugin interface {
	Register(s *grpc.Server)
}

type PluginManager struct {
	unixSocketFilePath string

	// Embedding these only to shorten access to fields.
	net.Listener
	server *grpc.Server

	plugin Plugin
}

// NewManager creates a new plugin manager.
func NewManager(plugin Plugin, unixSocketFilePath string) *PluginManager {
	return &PluginManager{
		unixSocketFilePath: unixSocketFilePath,
		plugin:             plugin,
	}
}

// ServeKMSRequests starts gRPC server or dies.
func (m *PluginManager) Start() (*grpc.Server, <-chan error) {
	errCh := make(chan error, 1)
	sendError := func(err error) {
		defer close(errCh)
		select {
		case errCh <- err:
		default:
		}
	}

	if err := m.cleanSockFile(); err != nil {
		sendError(fmt.Errorf("failed to cleanup socket file: %w", err))
		return nil, errCh
	}

	listener, err := net.Listen(netProtocol, m.unixSocketFilePath)
	if err != nil {
		sendError(fmt.Errorf("failed to create listener: %w", err))
		return nil, errCh
	}
	m.Listener = listener
	glog.Infof("Listening on unix domain socket: %s", m.unixSocketFilePath)

	m.server = grpc.NewServer()
	m.plugin.Register(m.server)

	go func() {
		defer m.cleanSockFile()
		sendError(m.server.Serve(m.Listener))
	}()

	return m.server, errCh
}

func (m *PluginManager) cleanSockFile() error {
	// @ implies the use of Linux socket namespace - no file on disk and nothing to clean-up.
	if strings.HasPrefix(m.unixSocketFilePath, "@") {
		return nil
	}

	err := os.Remove(m.unixSocketFilePath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete the socket file, error: %w", err)
	}
	return nil
}
