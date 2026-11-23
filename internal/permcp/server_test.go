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
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/perses/common/async"
	"github.com/perses/perses/pkg/client/config"
	"github.com/perses/spec/go/common"

	"github.com/perses/mcp-server/pkg/mcpapp"
	"github.com/perses/mcp-server/pkg/tools"
	"github.com/perses/mcp-server/pkg/tools/prometheus"
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

// TestNew checks that New registers the selected tools and sends the instructions on initialize.
func TestNew(t *testing.T) {
	withPanelsView(t, testPanelsView)
	task, err := New(Config{
		ReadOnly:         true,
		AllowedResources: []string{"dashboard", "prometheus"},
		PersesServer:     config.RestConfigClient{URL: common.MustParseURL("http://127.0.0.1:1")},
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	s, ok := task.(*server)
	if !ok {
		t.Fatalf("New() = %T, want *server", task)
	}
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := s.mcpServer.Connect(t.Context(), serverTransport, nil)
	if err != nil {
		t.Fatalf("connecting the server: %v", err)
	}
	t.Cleanup(func() { _ = serverSession.Close() })
	session, err := mcp.NewClient(&mcp.Implementation{Name: "test-client"}, nil).Connect(t.Context(), clientTransport, nil)
	if err != nil {
		t.Fatalf("connecting the client: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })

	toolList, err := session.ListTools(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	var registered []string
	for _, tool := range toolList.Tools {
		registered = append(registered, tool.Name)
	}
	// The read-only tools of the dashboards and of Prometheus only. The client doesn't declare MCP Apps: the app-only
	// perses_query_prometheus_data is not listed.
	want := []string{
		"perses_get_dashboard_by_name",
		"perses_get_prometheus_metric_metadata",
		"perses_list_dashboards",
		"perses_list_prometheus_label_names",
		"perses_list_prometheus_label_values",
		"perses_list_prometheus_metric_names",
		"perses_query_prometheus",
		"perses_render_panels",
	}
	slices.Sort(registered)
	if !slices.Equal(registered, want) {
		t.Errorf("registered tools = %v, want %v", registered, want)
	}

	wantInstructions, err := os.ReadFile("testdata/instructions/read-only-dashboard-prometheus.md")
	if err != nil {
		t.Fatal(err)
	}
	if got := session.InitializeResult().Instructions; got != string(wantInstructions) {
		t.Errorf("instructions =\n%s\nwant\n%s", got, wantInstructions)
	}
}

// TestMissingPanelsViewWarning checks that the missing view is reported, by Execute, only when the panels tools would
// have been registered.
func TestMissingPanelsViewWarning(t *testing.T) {
	tests := []struct {
		name      string
		view      string
		resources []string
		want      bool
	}{
		{name: "without the view", want: true},
		{name: "without the view and the prometheus resource", resources: []string{"dashboard"}},
		{name: "with the view", view: testPanelsView},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			withPanelsView(t, test.view)
			task, err := New(Config{
				AllowedResources: test.resources,
				PersesServer:     config.RestConfigClient{URL: common.MustParseURL("http://127.0.0.1:1")},
			})
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}
			if got := task.(*server).tools.withoutPanelsView; got != test.want {
				t.Errorf("withoutPanelsView = %v, want %v", got, test.want)
			}
		})
	}
}

// TestNewWithoutThePanelsView checks a server built without the MCP Apps views: the tools of the view are not
// registered, and the instructions don't mention them.
func TestNewWithoutThePanelsView(t *testing.T) {
	withPanelsView(t, "")
	task, err := New(Config{PersesServer: config.RestConfigClient{URL: common.MustParseURL("http://127.0.0.1:1")}})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	s := task.(*server)
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := s.mcpServer.Connect(t.Context(), serverTransport, nil)
	if err != nil {
		t.Fatalf("connecting the server: %v", err)
	}
	t.Cleanup(func() { _ = serverSession.Close() })
	// The client declares MCP Apps: it would list the app-only tool.
	options := &mcp.ClientOptions{Capabilities: &mcp.ClientCapabilities{}}
	options.Capabilities.AddExtension(mcpapp.ExtensionID, map[string]any{"mimeTypes": []any{mcpapp.ResourceMIMEType}})
	session, err := mcp.NewClient(&mcp.Implementation{Name: "test-client"}, options).Connect(t.Context(), clientTransport, nil)
	if err != nil {
		t.Fatalf("connecting the client: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })

	toolList, err := session.ListTools(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	var queryTool bool
	for _, tool := range toolList.Tools {
		switch tool.Name {
		case prometheus.RenderPanelsToolName, prometheus.QueryDataToolName:
			t.Errorf("%s is registered without its view", tool.Name)
		case prometheus.QueryToolName:
			queryTool = true
		}
	}
	if !queryTool {
		t.Errorf("%s is not registered", prometheus.QueryToolName)
	}
	if instructions := session.InitializeResult().Instructions; strings.Contains(instructions, prometheus.RenderPanelsToolName) {
		t.Errorf("the instructions mention %s:\n%s", prometheus.RenderPanelsToolName, instructions)
	}
	if _, err := session.ReadResource(t.Context(), &mcp.ReadResourceParams{URI: prometheus.PanelsViewURI}); err == nil {
		t.Errorf("the resource %s is registered without its view", prometheus.PanelsViewURI)
	}
}
