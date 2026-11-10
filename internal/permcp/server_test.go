// Copyright The Perses Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package permcp

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/perses/common/async"
	"github.com/perses/perses/pkg/client/config"
	"github.com/perses/spec/go/common"
)

// newServerWithBusyAddress returns an MCP server whose HTTP transport fails to start, because its address is already in use.
func newServerWithBusyAddress(t *testing.T) async.SimpleTask {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("unable to listen on a random port: %v", err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	task, err := New(Config{
		Transport:     HTTPTransport,
		ListenAddress: listener.Addr().String(),
		PersesServer:  config.RestConfigClient{URL: common.MustParseURL("http://localhost:8080")},
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return task
}

func TestServer_Execute(t *testing.T) {
	t.Run("the transport failure is returned and stops the application", func(t *testing.T) {
		task := newServerWithBusyAddress(t)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		err := task.Execute(ctx, cancel)

		if opErr, ok := errors.AsType[*net.OpError](err); !ok || opErr.Op != "listen" {
			t.Errorf("Execute() error = %v, want the listen error", err)
		}
		if ctx.Err() == nil {
			t.Error("the application is not stopped")
		}
	})
	t.Run("no error is returned when the application is stopping", func(t *testing.T) {
		task := newServerWithBusyAddress(t)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		if err := task.Execute(ctx, cancel); err != nil {
			t.Errorf("Execute() error = %v, want nil", err)
		}
	})
}
