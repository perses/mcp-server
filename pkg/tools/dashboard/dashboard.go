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
// Besides the usual list/get/create/update/delete tools, it returns compact summaries instead of whole
// dashboards, and it reports validation problems as structured issues located by JSON path.
package dashboard

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

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
		Description: "List the dashboards of a project: name, display name, version and panel count. " +
			"Use perses_get_dashboard_by_name to read one.",
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
				string(tools.ProjectResource): projectSchema("Project name"),
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
		return textResult(listed)
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
}

func (d *dashboard) Get() *tools.Tool {
	tool := &mcp.Tool{
		Name: "perses_get_dashboard_by_name",
		Description: "Get a dashboard by name in a project. By default returns a compact outline " +
			"(panel keys, plugins, placement, variables, layouts, URL) to decide what to change. " +
			"Set summary=false to get the complete dashboard JSON before read-modify-write, such as perses_update_dashboard. " +
			"Pass panels=[keys] to read those full panel definitions regardless of summary.",
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
				string(tools.ProjectResource): projectSchema("Project name"),
				tools.ResourceName:            nameSchema("Dashboard name"),
				"summary": {
					Type:        tools.SchemaTypeBoolean,
					Description: "Return a compact outline (default: true). Set false for complete dashboard JSON before read-modify-write.",
					Default:     json.RawMessage("true"),
				},
				"panels": {
					Type:        tools.SchemaTypeArray,
					Description: "Return only the panels with these keys (plus the summary).",
					Items:       &jsonschema.Schema{Type: tools.SchemaTypeString},
					MinItems:    new(1),
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
		switch {
		case len(input.Panels) > 0:
			result := panelsResult{Summary: dashboardutil.Summarize(dash, d.baseURL()), Panels: map[string]*dashboardSpec.Panel{}}
			for _, key := range input.Panels {
				if panel, ok := dash.Spec.Panels[key]; ok {
					result.Panels[key] = panel
				} else {
					result.Missing = append(result.Missing, key)
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
		Description: "Delete a dashboard from a specific project",
		InputSchema: &jsonschema.Schema{
			Type: tools.SchemaTypeObject,
			Properties: map[string]*jsonschema.Schema{
				string(tools.ProjectResource): projectSchema("Project name"),
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
