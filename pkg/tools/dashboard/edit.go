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
	"sort"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/perses/mcp-server/pkg/tools"
	"github.com/perses/mcp-server/pkg/tools/internal/dashboardutil"
	v1 "github.com/perses/perses/pkg/model/api/v1"
	dashboardSpec "github.com/perses/spec/go/dashboard"
)

// editProperties are the input properties shared by the incremental tools.
func editProperties(extra map[string]*jsonschema.Schema) map[string]*jsonschema.Schema {
	properties := map[string]*jsonschema.Schema{
		string(tools.ProjectResource): projectSchema("Project of the dashboard (metadata.name from perses_list_projects)"),
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

// Placement is an explicit position of a panel in a layout.
type Placement struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width,omitempty"`
	Height int `json:"height,omitempty"`
}

type SetPanelsInput struct {
	editInput
	Panels     json.RawMessage      `json:"panels,omitempty"`
	Order      []string             `json:"order,omitempty"`
	Layout     string               `json:"layout,omitempty"`
	Width      int                  `json:"width,omitempty"`
	Height     int                  `json:"height,omitempty"`
	Placements map[string]Placement `json:"placements,omitempty"`
	Remove     []string             `json:"remove,omitempty"`
}

func (d *dashboard) SetPanels() *tools.Tool {
	tool := &mcp.Tool{
		Name: "perses_set_dashboard_panels",
		Description: "Add, replace, move, resize or remove panels without resending the dashboard. Give at least one of " +
			"panels, placements or remove. Replaced panels keep their position. New panels take the first free spot of the " +
			"24-column Grid group named by layout (created if missing; last group by default), sized width x height " +
			"(default 12x6); placements set exact positions. Removals apply first. Validates before saving and returns " +
			"a summary of changes or field-level validation errors. See perses_get_plugin_schema for the spec of a panel or query " +
			"plugin kind. Explicit placements are not checked for overlap: when widening or moving a panel, also place the panels it would cover.",
		InputSchema: &jsonschema.Schema{
			Type: tools.SchemaTypeObject,
			Properties: editProperties(map[string]*jsonschema.Schema{
				"panels": {
					Types: []string{tools.SchemaTypeObject, tools.SchemaTypeString},
					Description: "Map of panel key to panel, e.g. {\"total\": {\"kind\": \"Panel\", \"spec\": {\"display\": {\"name\": \"Total\"}, " +
						"\"plugin\": {\"kind\": \"StatChart\", \"spec\": {\"calculation\": \"last\"}}, \"queries\": [{\"kind\": \"TimeSeriesQuery\", " +
						"\"spec\": {\"plugin\": {\"kind\": \"PrometheusTimeSeriesQuery\", \"spec\": {\"query\": \"count(up)\"}}}}]}}}. " +
						"A query uses the default datasource of its kind unless its plugin spec names one, e.g. \"datasource\": {\"kind\": \"PrometheusDatasource\", \"name\": \"<name>\"}. " +
						"Optional for placements-only edits. Keys must match " + tools.PatternResourceName + ".",
				},
				"order": {
					Type: tools.SchemaTypeArray,
					Description: "Panel keys to process first, in this order; remaining panel/placement keys are alphabetical. " +
						"Each key must appear in panels or placements. JSON object order is not preserved.",
					Items:       &jsonschema.Schema{Type: tools.SchemaTypeString},
					UniqueItems: true,
				},
				"layout": {
					Type:        tools.SchemaTypeString,
					Description: "Title of the Grid layout group receiving the new panels; created when it does not exist. Defaults to the last group.",
				},
				"width": {
					Type:        "integer",
					Description: "Width of the new panels in grid columns (1-24, default 12).",
					Minimum:     new(1.0),
					Maximum:     new(float64(dashboardutil.GridColumns)),
				},
				"height": {
					Type:        "integer",
					Description: "Height of the new panels in grid rows of 30px (default 6).",
					Minimum:     new(1.0),
				},
				"placements": {
					Type: tools.SchemaTypeObject,
					Description: "Optional exact position per panel key: {\"<key>\": {\"x\": 0, \"y\": 0, \"width\": 12, \"height\": 6}}. " +
						"Moves the existing layout item of the panel, keeping its group (or tab) and size unless 'layout', width or height is given; " +
						"unplaced panels go in the 'layout' group (default 12x6). " +
						"May reference an existing dashboard panel without including its definition in panels. " +
						"Rejected for a panel displayed in several groups or tabs; use perses_set_dashboard_layout for it.",
					AdditionalProperties: &jsonschema.Schema{
						Type: tools.SchemaTypeObject,
						Properties: map[string]*jsonschema.Schema{
							"x":      {Type: "integer", Minimum: new(0.0), Maximum: new(float64(dashboardutil.GridColumns - 1))},
							"y":      {Type: "integer", Minimum: new(0.0)},
							"width":  {Type: "integer", Minimum: new(1.0), Maximum: new(float64(dashboardutil.GridColumns))},
							"height": {Type: "integer", Minimum: new(1.0)},
						},
						Required: []string{"x", "y"},
					},
				},
				"remove": {
					Type: tools.SchemaTypeArray,
					Description: "Keys of panels to delete (panelList[].key from perses_get_dashboard_by_name, not titles), together " +
						"with their layout items; other panels do not move. Must not overlap panels or placements. Unknown keys are reported and ignored.",
					Items:       &jsonschema.Schema{Type: tools.SchemaTypeString, MinLength: new(1)},
					UniqueItems: true,
				},
			}),
			Required: []string{string(tools.ProjectResource), tools.ResourceName},
		},
		Annotations: editAnnotations("Adds or replaces panels of a dashboard in Perses"),
	}

	handler := func(_ context.Context, _ *mcp.CallToolRequest, input SetPanelsInput) (*mcp.CallToolResult, any, error) { //nolint:unparam
		var panels map[string]*dashboardSpec.Panel
		if len(input.Panels) > 0 {
			if issue := dashboardutil.DecodeValue(input.Panels, "panels", &panels); issue != nil {
				return invalidResult(dashboardutil.NewValidation([]dashboardutil.Issue{*issue}, nil))
			}
		}
		if len(panels) == 0 && len(input.Placements) == 0 && len(input.Remove) == 0 {
			return invalidResult(dashboardutil.NewValidation([]dashboardutil.Issue{{
				Path:    "panels",
				Message: "give at least one panel, placement or key to remove",
				Hint:    "use placements to move or resize existing panels, remove to delete panels, or perses_set_dashboard_layout to replace a layout group",
			}}, nil))
		}
		if len(panels) == 0 && len(input.Placements) == 0 && len(input.Order) > 0 {
			return nil, nil, fmt.Errorf("order: only applies to keys given in panels or placements")
		}
		for _, key := range input.Remove {
			_, inPanels := panels[key]
			_, inPlacements := input.Placements[key]
			if inPanels || inPlacements {
				return nil, nil, fmt.Errorf("remove: panel %q is also given in panels or placements; a key can be either removed or set", key)
			}
		}
		return d.edit(input.request(), func(dash *v1.Dashboard) ([]string, error) {
			changes := removePanels(dash, input.Remove)
			if len(panels) == 0 && len(input.Placements) == 0 {
				return changes, nil
			}
			set, err := setPanels(dash, panels, input)
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

func setPanels(dash *v1.Dashboard, panels map[string]*dashboardSpec.Panel, input SetPanelsInput) ([]string, error) {
	if dash.Spec.Panels == nil {
		dash.Spec.Panels = map[string]*dashboardSpec.Panel{}
	}
	keys := make([]string, 0, len(panels)+len(input.Placements))
	for key, panel := range panels {
		if panel == nil {
			return nil, fmt.Errorf("panels.%s is null: give a panel object, or list the key in remove to delete it", key)
		}
		if !dashboardutil.IsValidKey(key) {
			return nil, fmt.Errorf("panels.%s: the key must match %s", key, tools.PatternResourceName)
		}
		keys = append(keys, key)
	}
	for key := range input.Placements {
		if _, supplied := panels[key]; !supplied {
			if dash.Spec.Panels[key] == nil {
				return nil, fmt.Errorf("placements.%s: panel not found; supply its definition in panels or use an existing panel key", key)
			}
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	remaining := make(map[string]bool, len(keys))
	for _, key := range keys {
		remaining[key] = true
	}
	ordered := make([]string, 0, len(keys))
	for _, key := range input.Order {
		if !remaining[key] {
			return nil, fmt.Errorf("order: key %q must appear once and refer to panels or placements", key)
		}
		ordered = append(ordered, key)
		delete(remaining, key)
	}
	for _, key := range keys {
		if remaining[key] {
			ordered = append(ordered, key)
		}
	}
	keys = ordered

	placedItems, err := placeExplicitly(dash, keys, input)
	if err != nil {
		return nil, err
	}

	var changes []string
	for _, key := range keys {
		_, existed := dash.Spec.Panels[key]
		action := "moved"
		if panel, supplied := panels[key]; supplied {
			if panel.Kind == "" {
				panel.Kind = "Panel"
			}
			dash.Spec.Panels[key] = panel
			action = "added"
			if existed {
				action = "replaced"
			}
		}

		placement, placed := placedItems[key]
		switch {
		case placed:
			changes = append(changes, fmt.Sprintf("%s panel %s, %s", action, key, placement))
		case existed && dashboardutil.IsPlaced(dash.Spec.Layouts, key):
			changes = append(changes, fmt.Sprintf("%s panel %s, kept its position", action, key))
		default:
			layout := dashboardutil.EnsureLayout(&dash.Spec.Layouts, input.Layout)
			item, err := dashboardutil.PlacePanel(&dash.Spec.Layouts[layout], key, input.Width, input.Height)
			if err != nil {
				return nil, err
			}
			changes = append(changes, fmt.Sprintf("%s panel %s, placed in layout %d at x=%d y=%d w=%d h=%d", action, key, layout, item.X, item.Y, item.Width, item.Height))
		}
	}
	return changes, nil
}

// placeExplicitly reserves the explicit positions before auto-placement, freeing all old positions first. An
// already placed panel keeps its grid group or tab and its size unless the input overrides them. It returns the
// placement summary per panel key.
func placeExplicitly(dash *v1.Dashboard, keys []string, input SetPanelsInput) (map[string]string, error) {
	origins := make(map[string]dashboardutil.PanelLocation, len(input.Placements))
	for _, key := range keys {
		if _, ok := input.Placements[key]; !ok {
			continue
		}
		locations := dashboardutil.FindPanelItems(dash.Spec.Layouts, key)
		if len(locations) > 1 {
			names := make([]string, 0, len(locations))
			for _, location := range locations {
				names = append(names, dashboardutil.DescribePanelLocation(dash.Spec.Layouts, location))
			}
			return nil, fmt.Errorf("placements.%s: the panel appears in %d places (%s); a placement cannot tell which one to move, use perses_set_dashboard_layout to edit its layout items instead", key, len(locations), strings.Join(names, ", "))
		}
		if len(locations) == 1 {
			origins[key] = locations[0]
		}
	}
	for key := range origins {
		dashboardutil.RemovePanelItems(dash.Spec.Layouts, key)
	}
	placedItems := make(map[string]string, len(input.Placements))
	for _, key := range keys {
		placement, ok := input.Placements[key]
		if !ok {
			continue
		}
		width, height := placement.Width, placement.Height
		previous, found := origins[key]
		if found {
			if width <= 0 {
				width = previous.Item.Width
			}
			if height <= 0 {
				height = previous.Item.Height
			}
		}
		if found && input.Layout == "" && previous.Tab >= 0 {
			item, err := dashboardutil.PlacePanelInTab(&dash.Spec.Layouts[previous.Layout], previous.Tab, key, placement.X, placement.Y, width, height)
			if err != nil {
				return nil, err
			}
			placedItems[key] = fmt.Sprintf("placed in layout %d tab %d at x=%d y=%d w=%d h=%d", previous.Layout, previous.Tab, item.X, item.Y, item.Width, item.Height)
			continue
		}
		layout := -1
		if found && input.Layout == "" {
			if _, grid := dash.Spec.Layouts[previous.Layout].Spec.(*dashboardSpec.GridLayoutSpec); grid {
				layout = previous.Layout
			}
		}
		if layout < 0 {
			layout = dashboardutil.EnsureLayout(&dash.Spec.Layouts, input.Layout)
		}
		item, err := dashboardutil.PlacePanelAt(&dash.Spec.Layouts[layout], key, placement.X, placement.Y, width, height)
		if err != nil {
			return nil, err
		}
		placedItems[key] = fmt.Sprintf("placed in layout %d at x=%d y=%d w=%d h=%d", layout, item.X, item.Y, item.Width, item.Height)
	}
	return placedItems, nil
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
		items := dashboardutil.RemovePanelItems(dash.Spec.Layouts, key)
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
			"allowAllValue, allowMultiple}}; a TextVariable has no plugin: {kind: TextVariable, spec: {name, value}}. " +
			"Use them in queries as $name (with allowMultiple, match with =~\"$name\").",
		InputSchema: &jsonschema.Schema{
			Type: tools.SchemaTypeObject,
			Properties: editProperties(map[string]*jsonschema.Schema{
				"variables": {
					Types:       []string{tools.SchemaTypeArray, tools.SchemaTypeString},
					Description: "The variables to add or replace, as in spec.variables.",
					Items:       &jsonschema.Schema{Type: tools.SchemaTypeObject},
				},
				"remove": {
					Type: tools.SchemaTypeArray,
					Description: "Names (spec.name) of variables to delete; must not overlap the variables being set. If a remaining " +
						"variable or panel query still references $name, nothing is saved and the validation errors name the dependent; " +
						"remove dependents in the same call. Unknown names are reported and ignored.",
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
			if issue := dashboardutil.DecodeValue(input.Variables, "variables", &variables); issue != nil {
				return invalidResult(dashboardutil.NewValidation([]dashboardutil.Issue{*issue}, nil))
			}
		}
		if len(variables) == 0 && len(input.Remove) == 0 {
			return nil, nil, fmt.Errorf("give at least one variable to set or one name to remove")
		}
		removed := make(map[string]bool, len(input.Remove))
		for _, name := range input.Remove {
			removed[name] = true
		}
		for _, variable := range variables {
			if variable.Spec != nil && removed[variable.Spec.GetName()] {
				return nil, nil, fmt.Errorf("remove: variable %q is also given in variables; a name can be either removed or set", variable.Spec.GetName())
			}
		}
		return d.edit(input.request(), func(dash *v1.Dashboard) ([]string, error) {
			changes := removeVariables(dash, input.Remove)
			for _, variable := range variables {
				if variable.Spec == nil {
					return nil, fmt.Errorf("a variable has no spec")
				}
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
	Layout json.RawMessage `json:"layout,omitempty"`
	Index  *int            `json:"index,omitempty"`
	Title  string          `json:"title,omitempty"`
	Remove bool            `json:"remove,omitempty"`
}

func (d *dashboard) SetLayout() *tools.Tool {
	tool := &mcp.Tool{
		Name: "perses_set_dashboard_layout",
		Description: "Add, replace or remove one layout group (section) selected by index or title; an unmatched " +
			"layout is appended, and remove=true deletes the group but keeps its panels. To move panels between groups, " +
			"prefer perses_set_dashboard_panels with layout and placements. A layout is {kind: Grid, spec: {display: " +
			"{title, collapse: {open: true}}, items: [{x, y, width, height, content: {$ref: '#/spec/panels/<key>'}}]}}.",
		InputSchema: &jsonschema.Schema{
			Type: tools.SchemaTypeObject,
			Properties: editProperties(map[string]*jsonschema.Schema{
				"layout": {
					Types:       []string{tools.SchemaTypeObject, tools.SchemaTypeString},
					Description: "The layout group to add or to replace the selected one with, as in spec.layouts. Required unless remove=true.",
				},
				"index": {
					Type:        "integer",
					Description: "Index in spec.layouts of the group to replace or remove.",
					Minimum:     new(0.0),
				},
				"title": {
					Type:        tools.SchemaTypeString,
					Description: "Title of the group to replace or remove, when 'index' is not given. Defaults to the title of 'layout'.",
				},
				"remove": {
					Type:        tools.SchemaTypeBoolean,
					Description: "Remove the selected group instead of replacing it.",
					Default:     json.RawMessage("false"),
				},
			}),
			Required: []string{string(tools.ProjectResource), tools.ResourceName},
		},
		Annotations: editAnnotations("Adds, replaces or removes a layout group of a dashboard in Perses"),
	}
	// Repeating a removal by index deletes the next group, and repeating an untitled append adds a duplicate.
	tool.Annotations.IdempotentHint = false

	handler := func(_ context.Context, _ *mcp.CallToolRequest, input SetLayoutInput) (*mcp.CallToolResult, any, error) { //nolint:unparam
		var layout *dashboardSpec.Layout
		if len(input.Layout) > 0 && string(input.Layout) != "null" {
			layout = &dashboardSpec.Layout{}
			if issue := dashboardutil.DecodeValue(input.Layout, "layout", layout); issue != nil {
				return invalidResult(dashboardutil.NewValidation([]dashboardutil.Issue{*issue}, nil))
			}
		}
		if layout == nil && !input.Remove {
			return nil, nil, fmt.Errorf("layout is required unless remove=true")
		}
		return d.edit(input.request(), func(dash *v1.Dashboard) ([]string, error) {
			return setLayout(dash, layout, input)
		})
	}

	return &tools.Tool{
		MCPTool:      tool,
		IsWriteTool:  true,
		ResourceType: tools.DashboardResource,
		RegisterWith: func(server *mcp.Server) { mcp.AddTool(server, tool, handler) },
	}
}

func setLayout(dash *v1.Dashboard, layout *dashboardSpec.Layout, input SetLayoutInput) ([]string, error) {
	index := -1
	switch {
	case input.Index != nil:
		index = *input.Index
		if index >= len(dash.Spec.Layouts) {
			return nil, fmt.Errorf("layout index %d is out of range: the dashboard has %d layout(s)", index, len(dash.Spec.Layouts))
		}
	case input.Title != "":
		index = dashboardutil.FindLayoutByTitle(dash.Spec.Layouts, input.Title)
	case layout != nil:
		if title := dashboardutil.LayoutTitle(*layout); title != "" {
			index = dashboardutil.FindLayoutByTitle(dash.Spec.Layouts, title)
		}
	}

	if input.Remove {
		if index < 0 {
			return nil, fmt.Errorf("no layout matches: give the index or the title of the layout to remove")
		}
		title := dashboardutil.LayoutTitle(dash.Spec.Layouts[index])
		dash.Spec.Layouts = append(dash.Spec.Layouts[:index], dash.Spec.Layouts[index+1:]...)
		return []string{fmt.Sprintf("removed layout %d %q", index, title)}, nil
	}
	if index < 0 {
		dash.Spec.Layouts = append(dash.Spec.Layouts, *layout)
		return []string{fmt.Sprintf("added layout %d %q", len(dash.Spec.Layouts)-1, dashboardutil.LayoutTitle(*layout))}, nil
	}
	dash.Spec.Layouts[index] = *layout
	return []string{fmt.Sprintf("replaced layout %d %q", index, dashboardutil.LayoutTitle(*layout))}, nil
}
