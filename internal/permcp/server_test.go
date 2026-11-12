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
	"slices"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/perses/common/async"
	"github.com/perses/perses/pkg/client/config"
	"github.com/perses/spec/go/common"

	"github.com/perses/mcp-server/pkg/tools"
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

func TestSelectTools(t *testing.T) {
	tool := func(name string, resource tools.Resource, write bool) *tools.Tool {
		return &tools.Tool{MCPTool: &mcp.Tool{Name: name}, ResourceType: resource, IsWriteTool: write}
	}
	all := []*tools.Tool{
		tool("list_dashboards", tools.DashboardResource, false),
		tool("create_dashboard", tools.DashboardResource, true),
		tool("list_projects", tools.ProjectResource, false),
		tool("create_project", tools.ProjectResource, true),
	}
	tests := []struct {
		name                string
		tools               []*tools.Tool
		cfg                 Config
		wantSelected        []string
		wantSkippedResource []string
		wantSkippedReadOnly []string
	}{
		{name: "no filter", tools: all, wantSelected: []string{"list_dashboards", "create_dashboard", "list_projects", "create_project"}},
		{
			name:                "resources",
			tools:               all,
			cfg:                 Config{AllowedResources: []string{"dashboard"}},
			wantSelected:        []string{"list_dashboards", "create_dashboard"},
			wantSkippedResource: []string{"list_projects", "create_project"},
		},
		{
			name:                "read-only",
			tools:               all,
			cfg:                 Config{ReadOnly: true},
			wantSelected:        []string{"list_dashboards", "list_projects"},
			wantSkippedReadOnly: []string{"create_dashboard", "create_project"},
		},
		{
			// A write tool of a resource that is not allowed is skipped for its resource.
			name:                "resources and read-only",
			tools:               all,
			cfg:                 Config{ReadOnly: true, AllowedResources: []string{"dashboard"}},
			wantSelected:        []string{"list_dashboards"},
			wantSkippedResource: []string{"list_projects", "create_project"},
			wantSkippedReadOnly: []string{"create_dashboard"},
		},
		{name: "no tools", cfg: Config{ReadOnly: true}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			selection := selectTools(tt.tools, tt.cfg)
			for _, check := range []struct {
				kind string
				got  []*tools.Tool
				want []string
			}{
				{"selected", selection.selected, tt.wantSelected},
				{"skipped for their resource", selection.skippedResource, tt.wantSkippedResource},
				{"skipped in read-only mode", selection.skippedReadOnly, tt.wantSkippedReadOnly},
			} {
				if got := toolNames(check.got); !slices.Equal(got, check.want) {
					t.Errorf("%s = %v, want %v", check.kind, got, check.want)
				}
			}
		})
	}
}
