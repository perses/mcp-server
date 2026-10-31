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
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	apiclient "github.com/perses/perses/pkg/client/api"
	"github.com/perses/perses/pkg/client/perseshttp"
)

func TestRegisterToolsFiltering(t *testing.T) {
	tests := []struct {
		name     string
		config   Config
		expected map[string]bool
		count    int
	}{
		{
			name: "combined",
			expected: map[string]bool{
				"perses_get_dashboard_schema":       true,
				"perses_validate_dashboard":         true,
				"perses_validate_global_datasource": true,
				"perses_validate_global_variable":   true,
				"perses_set_dashboard_panels":       true,
				"perses_set_dashboard_variables":    true,
				"perses_set_dashboard_layout":       true,
				"perses_remove_dashboard_panels":    true,
				"perses_remove_dashboard_variables": true,
			},
		},
		{
			name:   "schema only",
			config: Config{AllowedResources: []string{"schema"}},
			count:  3,
			expected: map[string]bool{
				"perses_get_dashboard_schema": true,
				"perses_validate_dashboard":   false,
			},
		},
		{
			name:   "validate only",
			config: Config{AllowedResources: []string{"validate"}},
			count:  5,
			expected: map[string]bool{
				"perses_get_dashboard_schema":       false,
				"perses_validate_dashboard":         true,
				"perses_validate_datasource":        true,
				"perses_validate_global_datasource": true,
				"perses_validate_variable":          true,
				"perses_validate_global_variable":   true,
			},
		},
		{
			name:   "read only",
			config: Config{ReadOnly: true},
			expected: map[string]bool{
				"perses_get_dashboard_schema":       true,
				"perses_validate_dashboard":         true,
				"perses_validate_global_datasource": true,
				"perses_create_dashboard":           false,
				"perses_update_dashboard":           false,
				"perses_set_dashboard_panels":       false,
				"perses_remove_dashboard_panels":    false,
				"perses_set_dashboard_variables":    false,
				"perses_remove_dashboard_variables": false,
				"perses_set_dashboard_layout":       false,
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()
			s := &server{
				cfg:          test.config,
				persesClient: apiclient.NewWithClient(&perseshttp.RESTClient{}),
				mcpServer:    mcp.NewServer(&mcp.Implementation{Name: "test-server"}, nil),
			}
			s.registerTools()
			serverTransport, clientTransport := mcp.NewInMemoryTransports()
			serverSession, err := s.mcpServer.Connect(ctx, serverTransport, nil)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = serverSession.Close() })
			client, err := mcp.NewClient(&mcp.Implementation{Name: "test-client"}, nil).Connect(ctx, clientTransport, nil)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = client.Close() })
			result, err := client.ListTools(ctx, nil)
			if err != nil {
				t.Fatal(err)
			}
			names := map[string]bool{}
			for _, tool := range result.Tools {
				if names[tool.Name] {
					t.Errorf("duplicate tool: %s", tool.Name)
				}
				names[tool.Name] = true
				if test.config.ReadOnly && !tool.Annotations.ReadOnlyHint {
					t.Errorf("write tool registered in read-only mode: %s", tool.Name)
				}
				if strings.HasPrefix(tool.Name, "perses_validate_") && !tool.Annotations.ReadOnlyHint {
					t.Errorf("validator is not read-only: %s", tool.Name)
				}
			}
			if test.count > 0 && len(names) != test.count {
				t.Errorf("expected %d tools, got %d", test.count, len(names))
			}
			for name, want := range test.expected {
				if names[name] != want {
					t.Errorf("%s registered=%v, want %v", name, names[name], want)
				}
			}
			resourceList, err := client.ListResources(ctx, nil)
			if err != nil {
				t.Fatal(err)
			}
			hasGuide := false
			for _, resource := range resourceList.Resources {
				if resource.URI == "perses://guides/dashboard-model" {
					hasGuide = true
				}
			}
			wantGuide := len(test.config.AllowedResources) == 0
			for _, allowed := range test.config.AllowedResources {
				if allowed == "dashboard" {
					wantGuide = true
				}
			}
			if hasGuide != wantGuide {
				t.Errorf("dashboard guide registered=%v, want %v", hasGuide, wantGuide)
			}
		})
	}
}
