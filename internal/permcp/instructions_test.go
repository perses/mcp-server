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
	"fmt"
	"regexp"
	"testing"

	"github.com/perses/mcp-server/pkg/tools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServerInstructionsComposition(t *testing.T) {
	tests := []struct {
		name     string
		config   Config
		contains []string
		excludes []string
	}{
		{
			name: "all",
			contains: []string{
				"summary:true", "panels:[keys]", "spec.panels is a map", "#/spec/panels/<key>", "24-column grid",
				"perses_create_dashboard", "perses_set_dashboard_panels", "placements alone", "order controls auto-placement",
				"not reordering existing panels", "JSON key order", "perses_set_dashboard_variables",
				"perses_set_dashboard_layout", "perses_update_dashboard", "summary:false", "dry_run:true",
				"expected_version", "re-read and reapply", "best-effort", "not an atomic lock", "errors[].path",
				"perses_list_plugin_schemas", "perses_get_plugin_schema", "perses_get_dashboard_schema",
				"perses_validate_dashboard",
			},
		},
		{
			name:     "dashboard",
			config:   Config{Resources: "dashboard"},
			contains: []string{"summary:true", "placements alone", "expected_version", "dry_run:true"},
			excludes: []string{"perses_get_plugin_schema", "perses_list_plugin_schemas", "perses_get_dashboard_schema", "perses_validate_dashboard"},
		},
		{
			name:     "dashboard and schema",
			config:   Config{Resources: "dashboard,schema"},
			contains: []string{"perses_create_dashboard", "perses_get_plugin_schema"},
			excludes: []string{"perses_validate_dashboard"},
		},
		{
			name:     "read only",
			config:   Config{ReadOnly: true},
			contains: []string{"summary:true", "panels:[keys]", "perses_get_plugin_schema", "perses_validate_dashboard"},
			excludes: []string{"perses_create_", "perses_update_", "perses_set_", "dry_run", "expected_version", "placements", "skeleton", "summary:false"},
		},
		{
			name:     "dashboard read only",
			config:   Config{Resources: "dashboard", ReadOnly: true},
			contains: []string{"summary:true", "panels:[keys]"},
			excludes: []string{"perses_get_plugin_schema", "perses_validate_dashboard", "dry_run", "expected_version", "placements", "skeleton"},
		},
		{
			name:     "schema",
			config:   Config{Resources: "schema"},
			contains: []string{"perses_get_plugin_schema", "perses_list_plugin_schemas", "perses_get_dashboard_schema"},
			excludes: []string{"resources/read", "perses_get_dashboard_by_name", "perses_set_dashboard_panels", "perses_validate_dashboard", "dry_run"},
		},
		{
			name:     "validate",
			config:   Config{Resources: "validate"},
			contains: []string{"perses_validate_dashboard", "errors[].path"},
			excludes: []string{"resources/read", "perses_get_dashboard_by_name", "perses_set_dashboard_panels", "perses_get_plugin_schema", "dry_run"},
		},
		{
			name:     "unrelated toolset",
			config:   Config{Resources: "project"},
			contains: []string{"Perses tools operate on the configured Perses server."},
			excludes: []string{"resources/read", "dashboard", "schema", "dry_run"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := newTestServerSession(t, test.config)
			instructions := client.InitializeResult().Instructions
			require.NotEmpty(t, instructions)
			// A byte budget keeps the per-session instructions concise.
			assert.LessOrEqual(t, len(instructions), 2000)
			for _, text := range test.contains {
				assert.Contains(t, instructions, text)
			}
			for _, text := range test.excludes {
				assert.NotContains(t, instructions, text)
			}
		})
	}
}

func TestServerInstructionsOnlyRegisteredTools(t *testing.T) {
	toolName := regexp.MustCompile(`\bperses_[a-z_]+\b`)
	resourceFilters := []string{"", "dashboard,schema", "dashboard,validate", "schema,validate", "dashboard,schema,validate"}
	for _, resource := range tools.ValidResources {
		resourceFilters = append(resourceFilters, string(resource))
	}
	for _, filter := range resourceFilters {
		for _, readOnly := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/readOnly=%v", filter, readOnly), func(t *testing.T) {
				client := newTestServerSession(t, Config{Resources: filter, ReadOnly: readOnly})
				result, err := client.ListTools(context.Background(), nil)
				require.NoError(t, err)
				registered := map[string]bool{}
				for _, tool := range result.Tools {
					registered[tool.Name] = true
				}
				instructions := client.InitializeResult().Instructions
				for _, name := range toolName.FindAllString(instructions, -1) {
					assert.True(t, registered[name], "instructions mention unavailable tool %s", name)
				}
				assert.Nil(t, client.InitializeResult().Capabilities.Resources)
				assert.NotContains(t, instructions, "resources/read")
				if registered["perses_get_dashboard_by_name"] {
					assert.Contains(t, instructions, "spec.panels is a map")
					assert.Contains(t, instructions, "24-column grid")
					assert.Contains(t, instructions, "default datasource")
				}
				if readOnly {
					assert.NotContains(t, instructions, "dry_run")
					assert.NotContains(t, instructions, "expected_version")
				}
			})
		}
	}
}

func TestServerInstructionsEmptyInventory(t *testing.T) {
	assert.Equal(t, "Perses tools operate on the configured Perses server.", serverInstructions(nil))
}
