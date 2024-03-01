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
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"google.golang.org/grpc"
)

type fakePlugin struct{}

func (p *fakePlugin) Register(s *grpc.Server) {}

func TestSocket(t *testing.T) {
	t.Parallel()

	dir, err := os.MkdirTemp(os.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(dir); err != nil {
			t.Fatal(err)
		}
	})
	f := filepath.Join(dir, "listener.sock")

	pluginManager := NewManager(&fakePlugin{}, f)
	server, errCh := pluginManager.Start()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatal(err)
		}
	default:
	}

	fileInfo, err := os.Stat(f)
	if err != nil {
		t.Fatal(err)
	}

	if (fileInfo.Mode() & os.ModeSocket) != os.ModeSocket {
		t.Fatalf("got %v, wanted Srwxr-xr-x", fileInfo.Mode())
	}

	server.GracefulStop()

	select {
	case err := <-errCh:
		if err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
	}

	if _, err := os.Stat(f); err == nil {
		t.Fatal("expected socket to be cleaned-up by now")
	}
}
