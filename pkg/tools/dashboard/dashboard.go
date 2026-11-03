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

// Package dashboard provides the MCP tools managing Perses dashboards.
//
// Besides the usual list/get/create/update/delete tools, it provides incremental tools (set/remove panels,
// variables and layouts) so that a dashboard can be built step by step without re-sending the whole document,
// and it reports validation problems as structured issues located by JSON path.
package dashboard

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/perses/mcp-server/pkg/tools"
	"github.com/perses/mcp-server/pkg/tools/internal/dashboardutil"
	"github.com/perses/mcp-server/pkg/tools/resource"
	apiClient "github.com/perses/perses/pkg/client/api"
	v1Client "github.com/perses/perses/pkg/client/api/v1"
	v1 "github.com/perses/perses/pkg/model/api/v1"
	dashboardSpec "github.com/perses/spec/go/dashboard"
)

const (
	dryRunProperty          = "dry_run"
	expectedVersionProperty = "expected_version"
	includeDashboardProp    = "include_dashboard"
)

type dashboard struct {
	client apiClient.ClientInterface
}

// New returns the dashboard tools. The client must be the top-level Perses client: the tools use the v1 API and
// the validation API.
func New(client apiClient.ClientInterface) resource.Resource {
	return &dashboard{
		client: client,
	}
}

func (d *dashboard) GetTools() []*tools.Tool {
	return []*tools.Tool{
		d.List(),
		d.Get(),
		d.Create(),
		d.Update(),
		d.Delete(),
		d.SetPanels(),
		d.RemovePanels(),
		d.SetVariables(),
		d.RemoveVariables(),
		d.SetLayout(),
	}
}

func (d *dashboard) api(project string) v1Client.DashboardInterface {
	return d.client.V1().Dashboard(project)
}

// baseURL is the URL of the Perses server, used to build dashboard links; nil when unknown.
func (d *dashboard) baseURL() *url.URL {
	rest := d.client.RESTClient()
	if rest == nil || rest.BaseURL == nil {
		return nil
	}
	return rest.BaseURL.URL
}

func projectSchema(description string) *jsonschema.Schema {
	return &jsonschema.Schema{
		Type:        tools.SchemaTypeString,
		Description: description,
		MinLength:   new(1),
		MaxLength:   new(75),
		Pattern:     tools.PatternResourceName,
	}
}

func nameSchema(description string) *jsonschema.Schema {
	return projectSchema(description)
}

// dashboardSchema accepts a dashboard as a JSON object, or as JSON serialized in a string for backward
// compatibility.
func dashboardSchema(description string) *jsonschema.Schema {
	return &jsonschema.Schema{
		Types:       []string{tools.SchemaTypeObject, tools.SchemaTypeString},
		Description: description + " Pass a JSON object (preferred); a JSON document serialized in a string is also accepted.",
	}
}

func dryRunSchema() *jsonschema.Schema {
	return &jsonschema.Schema{
		Type:        tools.SchemaTypeBoolean,
		Description: "Validate the result against the Perses server and report what would happen without saving anything.",
		Default:     json.RawMessage("false"),
	}
}

func expectedVersionSchema() *jsonschema.Schema {
	return &jsonschema.Schema{
		Type: "integer",
		Description: "Optional metadata.version from your latest read; the tool fails without saving if it detects a " +
			"changed version before writing. This is a best-effort stale-write check, not an atomic lock: Perses does " +
			"not provide conditional writes, so a concurrent update after the check can still race.",
		Minimum: new(0.0),
	}
}

func includeDashboardSchema() *jsonschema.Schema {
	return &jsonschema.Schema{
		Type:        tools.SchemaTypeBoolean,
		Description: "Return the complete saved dashboard in addition to the summary. Off by default to keep the response small.",
		Default:     json.RawMessage("false"),
	}
}

// writeResult is the response of the tools modifying a dashboard: a summary rather than the whole dashboard.
type writeResult struct {
	// Action is what happened: created, updated, or for a dry run would_create, would_update.
	Action string `json:"action"`
	DryRun bool   `json:"dryRun,omitempty"`
	dashboardutil.Summary
	// Changes describes what the incremental tools did, e.g. the layout items added.
	Changes  []string              `json:"changes,omitempty"`
	Warnings []dashboardutil.Issue `json:"warnings,omitempty"`
	// Dashboard is the saved dashboard, when requested.
	Dashboard *v1.Dashboard `json:"dashboard,omitempty"`
}

func textResult(value any) (*mcp.CallToolResult, any, error) {
	text, err := dashboardutil.MarshalText(value)
	if err != nil {
		return nil, nil, fmt.Errorf("error marshalling the response: %w", err)
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: text}},
	}, nil, nil
}

// invalidResult reports the validation errors as a tool error whose content is the structured validation, so
// that the caller can fix the offending fields.
func invalidResult(validation *dashboardutil.Validation) (*mcp.CallToolResult, any, error) {
	text, err := dashboardutil.MarshalText(validation)
	if err != nil {
		return nil, nil, fmt.Errorf("error marshalling the validation: %w", err)
	}
	return &mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{&mcp.TextContent{Text: text}},
	}, nil, nil
}

// validate asks the Perses server to validate the dashboard and lints it locally. The returned validation is
// nil when the dashboard is valid and has no warning. An error means that the validation could not be performed.
func (d *dashboard) validate(dash *v1.Dashboard) (*dashboardutil.Validation, error) {
	errs, warnings := dashboardutil.Lint(dash)
	if err := d.client.Validate().Dashboard(dash); err != nil {
		message, ok := dashboardutil.IsBadRequest(err)
		if !ok {
			return nil, fmt.Errorf("error validating dashboard '%s' in project '%s': %w", dash.Metadata.Name, dash.Metadata.Project, err)
		}
		errs = append(dashboardutil.IssuesFromServerMessage(message, dash), errs...)
	}
	if len(errs) == 0 && len(warnings) == 0 {
		return nil, nil
	}
	return dashboardutil.NewValidation(errs, warnings), nil
}

// warningsOf returns the warnings of a validation that may be nil.
func warningsOf(validation *dashboardutil.Validation) []dashboardutil.Issue {
	if validation == nil {
		return nil
	}
	return validation.Warnings
}

// get fetches a dashboard, translating a 404 into a readable error.
func (d *dashboard) get(project, name string) (*v1.Dashboard, error) {
	dash, err := d.api(project).Get(name)
	if err != nil {
		if dashboardutil.IsNotFound(err) {
			return nil, notFoundError(name, project)
		}
		return nil, fmt.Errorf("error retrieving dashboard '%s' in project '%s': %w", name, project, err)
	}
	return dash, nil
}

func notFoundError(name, project string) error {
	return fmt.Errorf("dashboard '%s' does not exist in project '%s'", name, project)
}

type ListDashboardsInput struct {
	Project string `json:"project" jsonschema:"Project name to list dashboards from"`
}

// listedDashboard is the compact description of a dashboard in a list.
type listedDashboard struct {
	Name      string `json:"name"`
	Display   string `json:"display,omitempty"`
	Version   uint64 `json:"version"`
	Panels    int    `json:"panels"`
	Variables int    `json:"variables"`
	UpdatedAt string `json:"updatedAt,omitempty"`
}

func (d *dashboard) List() *tools.Tool {
	tool := &mcp.Tool{
		Name: "perses_list_dashboards",
		Description: "List the dashboards of a project. Returns {count, dashboards: [{name, display, version, panels, " +
			"variables, updatedAt}]}: count is the exact number of dashboards, so use it rather than counting the array, " +
			"which can be long (about 140 bytes per dashboard, hundreds in large projects) and may be truncated by your client. " +
			"Use it to find a dashboard name (display is the title shown in the UI) or the one with the most panels or variables; " +
			"use perses_get_dashboard_by_name to read one. Fails if the project does not exist (see perses_list_projects).",
		Annotations: &mcp.ToolAnnotations{
			DestructiveHint: new(false),
			IdempotentHint:  true,
			OpenWorldHint:   new(false),
			ReadOnlyHint:    true,
			Title:           "List dashboards for a specific project in Perses",
		},
		InputSchema: &jsonschema.Schema{
			Type: tools.SchemaTypeObject,
			Properties: map[string]*jsonschema.Schema{
				string(tools.ProjectResource): projectSchema("Project name (metadata.name from perses_list_projects)"),
			},
			Required: []string{string(tools.ProjectResource)},
		},
	}

	handler := func(_ context.Context, _ *mcp.CallToolRequest, input ListDashboardsInput) (*mcp.CallToolResult, any, error) { //nolint:unparam
		response, err := d.api(input.Project).List("")
		if err != nil {
			return nil, nil, fmt.Errorf("error retrieving dashboards: %w", err)
		}
		listed := make([]listedDashboard, 0, len(response))
		for _, dash := range response {
			item := listedDashboard{
				Name:      dash.Metadata.Name,
				Version:   dash.Metadata.Version,
				Panels:    len(dash.Spec.Panels),
				Variables: len(dash.Spec.Variables),
			}
			if dash.Spec.Display != nil {
				item.Display = dash.Spec.Display.Name
			}
			if !dash.Metadata.UpdatedAt.IsZero() {
				item.UpdatedAt = dash.Metadata.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z")
			}
			listed = append(listed, item)
		}
		return textResult(listResult{Count: len(listed), Dashboards: listed})
	}

	return &tools.Tool{
		MCPTool:      tool,
		IsWriteTool:  false,
		ResourceType: tools.DashboardResource,
		RegisterWith: func(server *mcp.Server) { mcp.AddTool(server, tool, handler) },
	}
}

type GetDashboardByNameInput struct {
	Project string   `json:"project" jsonschema:"Project name to retrieve the dashboard from"`
	Name    string   `json:"name" jsonschema:"Dashboard name to retrieve"`
	Summary *bool    `json:"summary,omitempty" jsonschema:"Return an outline instead of the full dashboard (default: true); set false for complete JSON before read-modify-write"`
	Panels  []string `json:"panels,omitempty" jsonschema:"Return only these panels"`
}

// panelsResult is the response of the get tool restricted to some panels.
type panelsResult struct {
	dashboardutil.Summary
	Panels  map[string]*dashboardSpec.Panel `json:"panels"`
	Missing []string                        `json:"missing,omitempty"`
	// Available maps every panel key of the dashboard to its title; only set when some requested keys are missing.
	Available map[string]string `json:"available,omitempty"`
}

// outlineResult is the outline returned when none of the requested panels exists, so the caller sees the keys.
type outlineResult struct {
	dashboardutil.Outline
	Missing []string `json:"missing"`
}

// listResult is the response of the list tool; count comes first so it survives client-side truncation.
type listResult struct {
	Count      int               `json:"count"`
	Dashboards []listedDashboard `json:"dashboards"`
}

func (d *dashboard) Get() *tools.Tool {
	tool := &mcp.Tool{
		Name: "perses_get_dashboard_by_name",
		Description: "Get one dashboard of a project. Call it first with only project and name: the default response is a compact outline with " +
			"variableList (per variable: name, kind, plugin, pluginSpec such as labelName/matchers or expr, defaultValue, allowMultiple, allowAllValue, or value for a TextVariable), " +
			"layoutList and panelList, where each panel has its key, title (name), plugin kind, query count and placement, but NOT its queries. " +
			"To read queries (e.g. PromQL), find the panel key by title in panelList and call again with panels=[key]; keys are the map keys of spec.panels " +
			"(e.g. 0_0 or cpu), never titles, so do not guess them. That response contains only the requested panels; when a key does not exist it is listed in missing " +
			"and available maps every key to its title; if none exists, the outline is returned with missing. Set summary=false only when you need the complete dashboard JSON (often 10+ KB), e.g. before perses_update_dashboard; " +
			"for small changes use the perses_set_dashboard_* / perses_remove_dashboard_* tools instead. Fails with 'does not exist' if the name is wrong; use perses_list_dashboards to find names.",
		Annotations: &mcp.ToolAnnotations{
			DestructiveHint: new(false),
			IdempotentHint:  true,
			OpenWorldHint:   new(false),
			ReadOnlyHint:    true,
			Title:           "Gets a dashboard by name in a specific project in Perses",
		},
		InputSchema: &jsonschema.Schema{
			Type: tools.SchemaTypeObject,
			Properties: map[string]*jsonschema.Schema{
				string(tools.ProjectResource): projectSchema("Project name (metadata.name from perses_list_projects)"),
				tools.ResourceName:            nameSchema("Dashboard name"),
				"summary": {
					Type:        tools.SchemaTypeBoolean,
					Description: "Return a compact outline (default: true). Set false for complete dashboard JSON before read-modify-write.",
					Default:     json.RawMessage("true"),
				},
				"panels": {
					Type:        tools.SchemaTypeArray,
					Description: "Omit (or pass []) on the first call to get the outline. Panel keys taken from panelList[].key of a previous outline (not titles, not placeholders) whose full definitions, including queries, to return. The response then omits the variable, panel and layout lists.",
					Items:       &jsonschema.Schema{Type: tools.SchemaTypeString},
				},
			},
			Required: []string{string(tools.ProjectResource), tools.ResourceName},
		},
	}

	handler := func(_ context.Context, _ *mcp.CallToolRequest, input GetDashboardByNameInput) (*mcp.CallToolResult, any, error) { //nolint:unparam
		dash, err := d.get(input.Project, input.Name)
		if err != nil {
			return nil, nil, err
		}
		var keys []string
		for _, key := range input.Panels {
			if strings.TrimSpace(key) != "" {
				keys = append(keys, key)
			}
		}
		found := false
		for _, key := range keys {
			_, ok := dash.Spec.Panels[key]
			found = found || ok
		}
		switch {
		case input.Panels != nil && len(keys) == 0:
			// An empty or blank selection asks for nothing specific: answer with the outline, never the full dashboard.
			return textResult(dashboardutil.OutlineOf(dash, d.baseURL()))
		case len(keys) > 0 && !found:
			// Guessed keys: answer with the outline so that the next call can use real keys.
			return textResult(outlineResult{Outline: dashboardutil.OutlineOf(dash, d.baseURL()), Missing: keys})
		case len(keys) > 0:
			result := panelsResult{Summary: dashboardutil.Summarize(dash, d.baseURL()), Panels: map[string]*dashboardSpec.Panel{}}
			for _, key := range keys {
				if panel, ok := dash.Spec.Panels[key]; ok {
					result.Panels[key] = panel
				} else {
					result.Missing = append(result.Missing, key)
				}
			}
			if len(result.Missing) > 0 {
				result.Available = map[string]string{}
				for key, panel := range dash.Spec.Panels {
					title := ""
					if panel.Spec.Display != nil {
						title = panel.Spec.Display.Name
					}
					result.Available[key] = title
				}
			}
			return textResult(result)
		case input.Summary == nil || *input.Summary:
			return textResult(dashboardutil.OutlineOf(dash, d.baseURL()))
		default:
			return textResult(dash)
		}
	}

	return &tools.Tool{
		MCPTool:      tool,
		IsWriteTool:  false,
		ResourceType: tools.DashboardResource,
		RegisterWith: func(server *mcp.Server) { mcp.AddTool(server, tool, handler) },
	}
}

type DeleteDashboardInput struct {
	Project string `json:"project" jsonschema:"Project name"`
	Name    string `json:"name" jsonschema:"Dashboard name to delete"`
}

func (d *dashboard) Delete() *tools.Tool {
	tool := &mcp.Tool{
		Name:        "perses_delete_dashboard",
		Description: "Permanently delete a dashboard from a project. Cannot be undone; to only remove some panels or variables use perses_remove_dashboard_panels or perses_remove_dashboard_variables.",
		InputSchema: &jsonschema.Schema{
			Type: tools.SchemaTypeObject,
			Properties: map[string]*jsonschema.Schema{
				string(tools.ProjectResource): projectSchema("Project name (metadata.name from perses_list_projects)"),
				tools.ResourceName:            nameSchema("Dashboard name"),
			},
			Required: []string{string(tools.ProjectResource), tools.ResourceName},
		},
		Annotations: &mcp.ToolAnnotations{
			DestructiveHint: new(true),
			IdempotentHint:  true,
			OpenWorldHint:   new(false),
			ReadOnlyHint:    false,
			Title:           "Deletes a dashboard from a specific project in Perses",
		},
	}

	handler := func(_ context.Context, _ *mcp.CallToolRequest, input DeleteDashboardInput) (*mcp.CallToolResult, any, error) { //nolint:unparam
		err := d.api(input.Project).Delete(input.Name)
		if err != nil {
			return nil, nil, fmt.Errorf("error deleting dashboard '%s' in project '%s': %w", input.Name, input.Project, err)
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: fmt.Sprintf("Dashboard '%s' deleted successfully from project '%s'", input.Name, input.Project),
				},
			},
		}, nil, nil
	}

	return &tools.Tool{
		MCPTool:      tool,
		IsWriteTool:  true,
		ResourceType: tools.DashboardResource,
		RegisterWith: func(server *mcp.Server) { mcp.AddTool(server, tool, handler) },
	}
}
