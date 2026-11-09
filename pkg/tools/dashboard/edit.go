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

package dashboard

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/perses/mcp-server/pkg/tools"
	v1 "github.com/perses/perses/pkg/model/api/v1"
	dashboardSpec "github.com/perses/spec/go/dashboard"
)

// editProperties are the input properties shared by the incremental tools.
func editProperties(extra map[string]*jsonschema.Schema) map[string]*jsonschema.Schema {
	properties := map[string]*jsonschema.Schema{
		string(tools.ProjectResource): projectSchema("Project of the dashboard"),
		tools.ResourceName:            nameSchema("Name of the existing dashboard to modify"),
		expectedVersionProperty:       expectedVersionSchema(),
		dryRunProperty:                dryRunSchema(),
		includeDashboardProp:          includeDashboardSchema(),
	}
	for key, schema := range extra {
		properties[key] = schema
	}
	return properties
}

// editInput is the common part of the inputs of the incremental tools.
type editInput struct {
	Project          string  `json:"project"`
	Name             string  `json:"name"`
	ExpectedVersion  *uint64 `json:"expected_version,omitempty"`
	DryRun           bool    `json:"dry_run,omitempty"`
	IncludeDashboard bool    `json:"include_dashboard,omitempty"`
}

func (e editInput) request() editRequest {
	return editRequest{
		project:         e.Project,
		name:            e.Name,
		expectedVersion: e.ExpectedVersion,
		dryRun:          e.DryRun,
		include:         e.IncludeDashboard,
	}
}

func editAnnotations(title string) *mcp.ToolAnnotations {
	return &mcp.ToolAnnotations{
		DestructiveHint: new(true),
		IdempotentHint:  true,
		OpenWorldHint:   new(false),
		ReadOnlyHint:    false,
		Title:           title,
	}
}

type SetPanelsInput struct {
	editInput
	Panels json.RawMessage `json:"panels,omitempty"`
	Group  string          `json:"group,omitempty"`
	Remove []string        `json:"remove,omitempty"`
}

func (d *dashboard) SetPanels() *tools.Tool {
	tool := &mcp.Tool{
		Name: "perses_set_dashboard_panels",
		Description: "Add, replace or remove panels without resending the dashboard. A replaced panel keeps its " +
			"position. New panels are appended, in key order, to the Grid group titled 'group' (created if missing; last " +
			"group by default), and that group is laid out again like the Perses SDK: every panel takes the size of the " +
			"group's first panel (default 2 per line, 8 high), so custom positions in that group are reset. " +
			"Removed panels lose their layout items; removals apply first. Reorder or resize a group with " +
			"perses_set_dashboard_layout; custom positions need perses_update_dashboard.",
		InputSchema: &jsonschema.Schema{
			Type: tools.SchemaTypeObject,
			Properties: editProperties(map[string]*jsonschema.Schema{
				"panels": {
					Type: tools.SchemaTypeObject,
					Description: "Map of panel key to panel: {\"<key>\": {\"kind\": \"Panel\", \"spec\": {\"display\": {\"name\": \"...\"}, " +
						"\"plugin\": {...}, \"queries\": [...]}}}.",
				},
				"group": {
					Type:        tools.SchemaTypeString,
					Description: "Title of the Grid group receiving the new panels. Defaults to the last group.",
				},
				"remove": {
					Type:        tools.SchemaTypeArray,
					Description: "Keys of panels to delete; must not overlap panels. Unknown keys are reported and ignored.",
					Items:       &jsonschema.Schema{Type: tools.SchemaTypeString, MinLength: new(1)},
					UniqueItems: true,
				},
			}),
			Required: []string{string(tools.ProjectResource), tools.ResourceName},
		},
		Annotations: editAnnotations("Adds, replaces or removes panels of a dashboard in Perses"),
	}

	handler := func(_ context.Context, _ *mcp.CallToolRequest, input SetPanelsInput) (*mcp.CallToolResult, any, error) { //nolint:unparam
		var panels map[string]*dashboardSpec.Panel
		if len(input.Panels) > 0 {
			if err := json.Unmarshal(input.Panels, &panels); err != nil {
				return nil, nil, err
			}
		}
		if len(panels) == 0 && len(input.Remove) == 0 {
			return nil, nil, fmt.Errorf("give at least one panel to set or one key to remove")
		}
		for key, panel := range panels {
			if panel == nil {
				return nil, nil, fmt.Errorf("panels.%s is null: give a panel object, or list the key in remove to delete it", key)
			}
		}
		for _, key := range input.Remove {
			if _, ok := panels[key]; ok {
				return nil, nil, fmt.Errorf("remove: panel %q is also given in panels; a key can be either removed or set", key)
			}
		}
		return d.edit(input.request(), func(dash *v1.Dashboard) ([]string, error) {
			changes := removePanels(dash, input.Remove)
			set, err := setPanels(dash, panels, input.Group)
			if err != nil {
				return nil, err
			}
			return append(changes, set...), nil
		})
	}

	return &tools.Tool{
		MCPTool:      tool,
		IsWriteTool:  true,
		ResourceType: tools.DashboardResource,
		RegisterWith: func(server *mcp.Server) { mcp.AddTool(server, tool, handler) },
	}
}

// setPanels adds or replaces the panels. Replaced panels keep their layout items; the others are appended to the
// group, which is laid out again.
func setPanels(dash *v1.Dashboard, panels map[string]*dashboardSpec.Panel, group string) ([]string, error) {
	if dash.Spec.Panels == nil {
		dash.Spec.Panels = map[string]*dashboardSpec.Panel{}
	}
	var changes, appended []string
	for _, key := range slices.Sorted(maps.Keys(panels)) {
		_, existed := dash.Spec.Panels[key]
		if panels[key].Kind == "" {
			panels[key].Kind = "Panel"
		}
		dash.Spec.Panels[key] = panels[key]
		switch {
		case existed && isPlaced(dash.Spec.Layouts, key):
			changes = append(changes, fmt.Sprintf("replaced panel %s, kept its position", key))
		case existed:
			changes = append(changes, fmt.Sprintf("replaced panel %s", key))
			appended = append(appended, key)
		default:
			changes = append(changes, fmt.Sprintf("added panel %s", key))
			appended = append(appended, key)
		}
	}
	if len(appended) == 0 {
		return changes, nil
	}
	index, err := findGroup(dash.Spec.Layouts, group)
	if err != nil {
		return nil, err
	}
	if index < 0 {
		spec := &dashboardSpec.GridLayoutSpec{}
		if group != "" {
			spec.Display = &dashboardSpec.GridLayoutDisplay{Title: group}
		}
		dash.Spec.Layouts = append(dash.Spec.Layouts, dashboardSpec.Layout{Kind: dashboardSpec.KindGridLayout, Spec: spec})
		index = len(dash.Spec.Layouts) - 1
	}
	grid := dash.Spec.Layouts[index].Spec.(*dashboardSpec.GridLayoutSpec)
	keys, perLine, height := groupPanels(grid)
	items := gridItems(append(keys, appended...), perLine, height)
	keepRepeats(items, dash.Spec.Layouts[index:index+1])
	grid.Items = items
	return append(changes, describeGroup(index, dash.Spec.Layouts[index])), nil
}

// describeGroup reports the layout of a group, e.g. `group 1 "CPU": a x=0 y=0, b x=12 y=0 (w=12 h=8)`.
func describeGroup(index int, layout dashboardSpec.Layout) string {
	grid := layout.Spec.(*dashboardSpec.GridLayoutSpec)
	positions := make([]string, 0, len(grid.Items))
	for _, item := range grid.Items {
		positions = append(positions, fmt.Sprintf("%s x=%d y=%d", panelKeyOf(item), item.X, item.Y))
	}
	description := fmt.Sprintf("group %d %q: %s", index, layoutTitle(layout), strings.Join(positions, ", "))
	if len(grid.Items) > 0 {
		description += fmt.Sprintf(" (w=%d h=%d)", grid.Items[0].Width, grid.Items[0].Height)
	}
	return description
}

// removePanels deletes the panels and the layout items displaying them; unknown keys are reported.
func removePanels(dash *v1.Dashboard, keys []string) []string {
	var changes []string
	for _, key := range keys {
		if _, ok := dash.Spec.Panels[key]; !ok {
			changes = append(changes, fmt.Sprintf("panel %s not found", key))
			continue
		}
		delete(dash.Spec.Panels, key)
		items := removePanelItems(dash.Spec.Layouts, map[string]bool{key: true}, true)
		changes = append(changes, fmt.Sprintf("removed panel %s and %d layout item(s)", key, items))
	}
	return changes
}

type SetVariablesInput struct {
	editInput
	Variables json.RawMessage `json:"variables,omitempty"`
	Remove    []string        `json:"remove,omitempty"`
}

func (d *dashboard) SetVariables() *tools.Tool {
	tool := &mcp.Tool{
		Name: "perses_set_dashboard_variables",
		Description: "Add, replace or remove variables of an existing dashboard. A variable with the same spec.name is " +
			"replaced in place, others are appended; removals apply first. Variables are {kind: TextVariable|ListVariable, " +
			"spec: {name, display: {name}, plugin: {kind: e.g. PrometheusLabelValuesVariable, spec: {labelName, matchers}}, " +
			"allowAllValue, allowMultiple}}; use them in queries as $name.",
		InputSchema: &jsonschema.Schema{
			Type: tools.SchemaTypeObject,
			Properties: editProperties(map[string]*jsonschema.Schema{
				"variables": {
					Type:        tools.SchemaTypeArray,
					Description: "The variables to add or replace, as in spec.variables.",
					Items:       &jsonschema.Schema{Type: tools.SchemaTypeObject},
				},
				"remove": {
					Type: tools.SchemaTypeArray,
					Description: "Names (spec.name) of variables to delete; must not overlap the variables being set. " +
						"Unknown names are reported and ignored.",
					Items:       &jsonschema.Schema{Type: tools.SchemaTypeString, MinLength: new(1)},
					UniqueItems: true,
				},
			}),
			Required: []string{string(tools.ProjectResource), tools.ResourceName},
		},
		Annotations: editAnnotations("Adds, replaces or removes variables of a dashboard in Perses"),
	}

	handler := func(_ context.Context, _ *mcp.CallToolRequest, input SetVariablesInput) (*mcp.CallToolResult, any, error) { //nolint:unparam
		var variables []dashboardSpec.Variable
		if len(input.Variables) > 0 {
			if err := json.Unmarshal(input.Variables, &variables); err != nil {
				return nil, nil, err
			}
		}
		if len(variables) == 0 && len(input.Remove) == 0 {
			return nil, nil, fmt.Errorf("give at least one variable to set or one name to remove")
		}
		removed := make(map[string]bool, len(input.Remove))
		for _, name := range input.Remove {
			removed[name] = true
		}
		seen := make(map[string]bool, len(variables))
		for _, variable := range variables {
			if variable.Spec == nil {
				return nil, nil, fmt.Errorf("a variable has no spec")
			}
			name := variable.Spec.GetName()
			if removed[name] {
				return nil, nil, fmt.Errorf("remove: variable %q is also given in variables; a name can be either removed or set", name)
			}
			if seen[name] {
				return nil, nil, fmt.Errorf("variables: %q is given twice", name)
			}
			seen[name] = true
		}
		return d.edit(input.request(), func(dash *v1.Dashboard) ([]string, error) {
			changes := removeVariables(dash, input.Remove)
			for _, variable := range variables {
				name := variable.Spec.GetName()
				if index := variableIndex(dash, name); index >= 0 {
					dash.Spec.Variables[index] = variable
					changes = append(changes, fmt.Sprintf("replaced variable %s", name))
				} else {
					dash.Spec.Variables = append(dash.Spec.Variables, variable)
					changes = append(changes, fmt.Sprintf("added variable %s", name))
				}
			}
			return changes, nil
		})
	}

	return &tools.Tool{
		MCPTool:      tool,
		IsWriteTool:  true,
		ResourceType: tools.DashboardResource,
		RegisterWith: func(server *mcp.Server) { mcp.AddTool(server, tool, handler) },
	}
}

func variableIndex(dash *v1.Dashboard, name string) int {
	for i, variable := range dash.Spec.Variables {
		if variable.Spec != nil && variable.Spec.GetName() == name {
			return i
		}
	}
	return -1
}

// removeVariables deletes the variables by name; unknown names are reported.
func removeVariables(dash *v1.Dashboard, names []string) []string {
	var changes []string
	for _, name := range names {
		index := variableIndex(dash, name)
		if index < 0 {
			changes = append(changes, fmt.Sprintf("variable %s not found", name))
			continue
		}
		dash.Spec.Variables = append(dash.Spec.Variables[:index], dash.Spec.Variables[index+1:]...)
		changes = append(changes, fmt.Sprintf("removed variable %s", name))
	}
	return changes
}

type SetLayoutInput struct {
	editInput
	Title         string   `json:"title"`
	Panels        []string `json:"panels,omitempty"`
	PanelsPerLine int      `json:"panels_per_line,omitempty"`
	PanelHeight   int      `json:"panel_height,omitempty"`
	Collapsed     *bool    `json:"collapsed,omitempty"`
	Remove        bool     `json:"remove,omitempty"`
}

func (d *dashboard) SetLayout() *tools.Tool {
	tool := &mcp.Tool{
		Name: "perses_set_dashboard_layout",
		Description: "Define, replace or remove one Grid panel group (section), matched by title, like the Perses SDK " +
			"AddPanelGroup: the listed panels are laid out in order, panels_per_line per row of 24 columns, all " +
			"panel_height high. Listed panels move out of other Grid groups; panels left out of the group are no longer " +
			"displayed. remove=true deletes the group but keeps its panels. Tabs layouts are left untouched. Custom " +
			"positions need perses_update_dashboard.",
		InputSchema: &jsonschema.Schema{
			Type: tools.SchemaTypeObject,
			Properties: editProperties(map[string]*jsonschema.Schema{
				"title": {
					Type:        tools.SchemaTypeString,
					Description: "Title of the group; an existing Grid group with this title (exact, else ignoring case) is replaced, otherwise one is appended.",
					MinLength:   new(1),
				},
				"panels": {
					Type:        tools.SchemaTypeArray,
					Description: "Keys of existing panels, in display order. Required unless remove=true.",
					Items:       &jsonschema.Schema{Type: tools.SchemaTypeString, MinLength: new(1)},
					UniqueItems: true,
				},
				"panels_per_line": {
					Type:        "integer",
					Description: "Panels per row; width is 24/panels_per_line rounded down. Defaults to the current group's, else 2.",
					Minimum:     new(1.0),
					Maximum:     new(float64(gridColumns)),
				},
				"panel_height": {
					Type:        "integer",
					Description: "Panel height in rows of 30px. Defaults to the current group's, else 8.",
					Minimum:     new(1.0),
				},
				"collapsed": {
					Type:        tools.SchemaTypeBoolean,
					Description: "Make the group collapsible, collapsed when true. Defaults to the current group's setting.",
				},
				"remove": {
					Type:        tools.SchemaTypeBoolean,
					Description: "Remove the group instead of setting it; no other group option may be given.",
					Default:     json.RawMessage("false"),
				},
			}),
			Required: []string{string(tools.ProjectResource), tools.ResourceName, "title"},
		},
		Annotations: editAnnotations("Sets or removes a panel group of a dashboard in Perses"),
	}

	handler := func(_ context.Context, _ *mcp.CallToolRequest, input SetLayoutInput) (*mcp.CallToolResult, any, error) { //nolint:unparam
		if len(input.Panels) == 0 && !input.Remove {
			return nil, nil, fmt.Errorf("panels is required unless remove=true")
		}
		if input.Remove && (len(input.Panels) > 0 || input.PanelsPerLine > 0 || input.PanelHeight > 0 || input.Collapsed != nil) {
			return nil, nil, fmt.Errorf("remove=true deletes the group: panels, panels_per_line, panel_height and collapsed cannot be given with it")
		}
		return d.edit(input.request(), func(dash *v1.Dashboard) ([]string, error) {
			return setLayout(dash, input)
		})
	}

	return &tools.Tool{
		MCPTool:      tool,
		IsWriteTool:  true,
		ResourceType: tools.DashboardResource,
		RegisterWith: func(server *mcp.Server) { mcp.AddTool(server, tool, handler) },
	}
}

func setLayout(dash *v1.Dashboard, input SetLayoutInput) ([]string, error) {
	index, err := findGroup(dash.Spec.Layouts, input.Title)
	if err != nil {
		return nil, err
	}
	if input.Remove {
		if index < 0 {
			return nil, fmt.Errorf("no Grid group is titled %q", input.Title)
		}
		removed := dash.Spec.Layouts[index]
		previous, _, _ := groupPanels(removed.Spec.(*dashboardSpec.GridLayoutSpec))
		dash.Spec.Layouts = slices.Delete(dash.Spec.Layouts, index, index+1)
		changes := []string{fmt.Sprintf("removed group %d %q", index, layoutTitle(removed))}
		return append(changes, hiddenPanels(dash, previous, nil)...), nil
	}
	for _, key := range input.Panels {
		if dash.Spec.Panels[key] == nil {
			return nil, fmt.Errorf("panel %q not found; available panels: %s", key, strings.Join(slices.Sorted(maps.Keys(dash.Spec.Panels)), ", "))
		}
	}

	grid := &dashboardSpec.GridLayoutSpec{Display: &dashboardSpec.GridLayoutDisplay{Title: input.Title}}
	perLine, height := defaultPanelsPerLine, defaultPanelHeight
	var previous []string
	if index >= 0 {
		current := dash.Spec.Layouts[index].Spec.(*dashboardSpec.GridLayoutSpec)
		previous, perLine, height = groupPanels(current)
		grid.RepeatVariable = current.RepeatVariable
		if current.Display != nil {
			grid.Display.Collapse = current.Display.Collapse
		}
	}
	if input.PanelsPerLine > 0 {
		perLine = min(input.PanelsPerLine, gridColumns)
	}
	if input.PanelHeight > 0 {
		height = input.PanelHeight
	}
	if input.Collapsed != nil {
		grid.Display.Collapse = &dashboardSpec.GridLayoutCollapse{Open: !*input.Collapsed}
	}

	grid.Items = gridItems(input.Panels, perLine, height)
	keepRepeats(grid.Items, dash.Spec.Layouts)
	listed := make(map[string]bool, len(input.Panels))
	for _, key := range input.Panels {
		listed[key] = true
	}
	var changes []string
	others := dash.Spec.Layouts
	if index >= 0 {
		others = slices.Delete(slices.Clone(others), index, index+1)
	}
	if moved := removePanelItems(others, listed, false); moved > 0 {
		changes = append(changes, fmt.Sprintf("moved %d layout item(s) out of other Grid groups", moved))
	}
	layout := dashboardSpec.Layout{Kind: dashboardSpec.KindGridLayout, Spec: grid}
	if index < 0 {
		dash.Spec.Layouts = append(dash.Spec.Layouts, layout)
		index = len(dash.Spec.Layouts) - 1
	} else {
		dash.Spec.Layouts[index] = layout
	}
	changes = append(changes, describeGroup(index, layout))
	return append(changes, hiddenPanels(dash, previous, listed)...), nil
}

// hiddenPanels reports the panels of a group, except the listed ones, that no layout item displays anymore.
func hiddenPanels(dash *v1.Dashboard, previous []string, listed map[string]bool) []string {
	var hidden []string
	for _, key := range previous {
		if !listed[key] && !isPlaced(dash.Spec.Layouts, key) {
			hidden = append(hidden, key)
		}
	}
	if len(hidden) == 0 {
		return nil
	}
	return []string{fmt.Sprintf("panels no longer displayed: %s", strings.Join(hidden, ", "))}
}
