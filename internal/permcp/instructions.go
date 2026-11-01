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
	"strings"

	"github.com/perses/common/set"
	"github.com/perses/mcp-server/pkg/tools"
	"github.com/perses/mcp-server/pkg/tools/dashboard"
)

// serverInstructions uses the filtered inventory so it never recommends unavailable tools.
func serverInstructions(registered []*tools.Tool) string {
	names := set.New[string]()
	dashboardWrites := false
	for _, tool := range registered {
		names.Add(tool.MCPTool.Name)
		if tool.ResourceType == tools.DashboardResource && tool.IsWriteTool {
			dashboardWrites = true
		}
	}

	instructions := []string{"Perses tools operate on the configured Perses server."}
	rules := []struct {
		requires []string
		text     string
	}{
		{
			[]string{"perses_get_dashboard_by_name"},
			"Read dashboards with perses_get_dashboard_by_name using summary:true (default); use panels:[keys] " +
				"only for the definitions you need. Model: spec.panels is a map; only panels referenced by " +
				"spec.layouts render (#/spec/panels/<key>, 24-column grid). Omit query spec.datasource to use the " +
				"default datasource. If your client supports MCP resources, read the full guide with resources/read " +
				"(uri: " + dashboard.GuideURI + "); otherwise use these inline rules.",
		},
		{
			[]string{"perses_create_dashboard", "perses_set_dashboard_panels"},
			"Create a skeleton with perses_create_dashboard, then add panels incrementally.",
		},
		{
			[]string{"perses_set_dashboard_panels"},
			"Prefer perses_set_dashboard_panels for panel edits: placements alone move/resize existing panels; " +
				"order controls auto-placement, not reordering existing panels. Do not rely on JSON key order.",
		},
		{
			[]string{"perses_set_dashboard_variables"},
			"Use perses_set_dashboard_variables for incremental variable edits.",
		},
		{
			[]string{"perses_set_dashboard_layout"},
			"Use perses_set_dashboard_layout for layout-group changes.",
		},
		{
			[]string{"perses_update_dashboard", "perses_get_dashboard_by_name"},
			"Use perses_update_dashboard only for full replacement after reading with summary:false.",
		},
		{
			[]string{"perses_list_plugin_schemas", "perses_get_plugin_schema", "perses_get_dashboard_schema"},
			"Discover installed plugins with perses_list_plugin_schemas; use perses_get_plugin_schema for needed " +
				"plugin specs. Fetch the large perses_get_dashboard_schema only if you need the full schema.",
		},
		{
			[]string{"perses_validate_dashboard"},
			"Use perses_validate_dashboard to validate without saving; fix errors at errors[].path using message/hint.",
		},
	}
	for _, rule := range rules {
		enabled := true
		for _, name := range rule.requires {
			if !names.Contains(name) {
				enabled = false
				break
			}
		}
		if enabled {
			instructions = append(instructions, rule.text)
		}
	}
	if dashboardWrites {
		instructions = append(instructions,
			"For dashboard create/edit calls, use dry_run:true to validate without saving. For edits, pass "+
				"expected_version from your latest read; on mismatch re-read and reapply. This check is best-effort, "+
				"not an atomic lock. Fix validation errors at errors[].path using message/hint.")
	}
	return strings.Join(instructions, "\n")
}
