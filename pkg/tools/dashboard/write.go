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

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/perses/mcp-server/pkg/tools"
	"github.com/perses/mcp-server/pkg/tools/internal/dashboardutil"
	v1 "github.com/perses/perses/pkg/model/api/v1"
)

type CreateDashboardInput struct {
	Project          string          `json:"project"`
	Dashboard        json.RawMessage `json:"dashboard"`
	DryRun           bool            `json:"dry_run,omitempty"`
	IncludeDashboard bool            `json:"include_dashboard,omitempty"`
}

func (d *dashboard) Create() *tools.Tool {
	tool := &mcp.Tool{
		Name: "perses_create_dashboard",
		Description: "Create a dashboard in a project from its complete JSON definition. The dashboard is validated " +
			"by the Perses server first; on failure the result lists the errors with the JSON path of each offending " +
			"field (e.g. spec.panels.cpu.spec.plugin.spec.calculation) so that only that field needs fixing. " +
			"Returns a short summary (name, version, counts, URL), not the dashboard. " +
			"Fails if a dashboard with the same name exists; use perses_update_dashboard to change it. " +
			"Set dry_run=true to only validate.",
		InputSchema: &jsonschema.Schema{
			Type: tools.SchemaTypeObject,
			Properties: map[string]*jsonschema.Schema{
				string(tools.ProjectResource): projectSchema("Project to create the dashboard in; fills metadata.project when omitted from the dashboard."),
				"dashboard":                   dashboardSchema("The dashboard: {kind: Dashboard, metadata: {name}, spec: {display, variables, panels, layouts, duration}}."),
				dryRunProperty:                dryRunSchema(),
				includeDashboardProp:          includeDashboardSchema(),
			},
			Required: []string{string(tools.ProjectResource), "dashboard"},
		},
		Annotations: &mcp.ToolAnnotations{
			DestructiveHint: new(false),
			IdempotentHint:  false,
			OpenWorldHint:   new(false),
			ReadOnlyHint:    false,
			Title:           "Creates a new dashboard in a specific project in Perses",
		},
	}

	handler := func(_ context.Context, _ *mcp.CallToolRequest, input CreateDashboardInput) (*mcp.CallToolResult, any, error) { //nolint:unparam
		dash, issues := dashboardutil.Decode(input.Dashboard, input.Project)
		if len(issues) > 0 {
			return invalidResult(dashboardutil.NewValidation(issues, nil))
		}
		validation, err := d.validate(dash)
		if err != nil {
			return nil, nil, err
		}
		if validation != nil && !validation.Valid {
			return invalidResult(validation)
		}

		result := writeResult{Action: "created", DryRun: input.DryRun, Warnings: warningsOf(validation)}
		if input.DryRun {
			result.Action = "would_create"
			_, err := d.api(input.Project).Get(dash.Metadata.Name)
			switch {
			case err == nil:
				return nil, nil, alreadyExistsError(dash.Metadata.Name, input.Project)
			case dashboardutil.IsNotFound(err):
			default:
				return nil, nil, fmt.Errorf("error checking whether dashboard '%s' exists in project '%s': %w", dash.Metadata.Name, input.Project, err)
			}
			result.Summary = dashboardutil.Summarize(dash, d.baseURL())
			return textResult(result)
		}

		saved, err := d.api(input.Project).Create(dash)
		switch {
		case err == nil:
		case dashboardutil.IsConflict(err):
			return nil, nil, alreadyExistsError(dash.Metadata.Name, input.Project)
		default:
			return nil, nil, fmt.Errorf("error creating dashboard '%s' in project '%s': %w", dash.Metadata.Name, input.Project, err)
		}
		return d.saved(result, saved, input.IncludeDashboard)
	}

	return &tools.Tool{
		MCPTool:      tool,
		IsWriteTool:  true,
		ResourceType: tools.DashboardResource,
		RegisterWith: func(server *mcp.Server) { mcp.AddTool(server, tool, handler) },
	}
}

func alreadyExistsError(name, project string) error {
	return fmt.Errorf("dashboard '%s' already exists in project '%s': use perses_update_dashboard to change it", name, project)
}

// saved completes the result with the summary of the dashboard returned by the server.
func (d *dashboard) saved(result writeResult, saved *v1.Dashboard, include bool) (*mcp.CallToolResult, any, error) {
	result.Summary = dashboardutil.Summarize(saved, d.baseURL())
	if include {
		result.Dashboard = saved
	}
	return textResult(result)
}

type UpdateDashboardInput struct {
	Project          string          `json:"project"`
	Dashboard        json.RawMessage `json:"dashboard"`
	ExpectedVersion  *uint64         `json:"expected_version,omitempty"`
	DryRun           bool            `json:"dry_run,omitempty"`
	IncludeDashboard bool            `json:"include_dashboard,omitempty"`
}

func (d *dashboard) Update() *tools.Tool {
	tool := &mcp.Tool{
		Name: "perses_update_dashboard",
		Description: "Replace an existing dashboard with its complete JSON definition (the dashboard must exist). " +
			"Validation errors are returned with the JSON path of each offending field; the response is a short " +
			"summary.",
		InputSchema: &jsonschema.Schema{
			Type: tools.SchemaTypeObject,
			Properties: map[string]*jsonschema.Schema{
				string(tools.ProjectResource): projectSchema("Project of the dashboard"),
				"dashboard":                   dashboardSchema("The complete dashboard; metadata.name identifies the dashboard to replace."),
				expectedVersionProperty:       expectedVersionSchema(),
				dryRunProperty:                dryRunSchema(),
				includeDashboardProp:          includeDashboardSchema(),
			},
			Required: []string{string(tools.ProjectResource), "dashboard"},
		},
		Annotations: &mcp.ToolAnnotations{
			DestructiveHint: new(true),
			IdempotentHint:  true,
			OpenWorldHint:   new(false),
			ReadOnlyHint:    false,
			Title:           "Updates an existing dashboard in a specific project in Perses",
		},
	}

	handler := func(_ context.Context, _ *mcp.CallToolRequest, input UpdateDashboardInput) (*mcp.CallToolResult, any, error) { //nolint:unparam
		dash, issues := dashboardutil.Decode(input.Dashboard, input.Project)
		if len(issues) > 0 {
			return invalidResult(dashboardutil.NewValidation(issues, nil))
		}
		// Perses checks the read and update permissions independently: read the dashboard only when needed, so that
		// an update-only role can still update. Without the read, the PUT itself rejects a missing dashboard.
		if input.DryRun || input.ExpectedVersion != nil {
			current, err := d.get(input.Project, dash.Metadata.Name)
			if err != nil {
				return nil, nil, err
			}
			if err := checkVersion(current, input.ExpectedVersion); err != nil {
				return nil, nil, err
			}
		}
		return d.save(editRequest{
			project: input.Project,
			name:    dash.Metadata.Name,
			dryRun:  input.DryRun,
			include: input.IncludeDashboard,
		}, dash, nil)
	}

	return &tools.Tool{
		MCPTool:      tool,
		IsWriteTool:  true,
		ResourceType: tools.DashboardResource,
		RegisterWith: func(server *mcp.Server) { mcp.AddTool(server, tool, handler) },
	}
}

// checkVersion implements the optimistic locking of the update tools: the Perses server does not check versions,
// so the tools compare the version read before modifying with the one expected by the caller.
func checkVersion(current *v1.Dashboard, expected *uint64) error {
	if expected != nil && current.Metadata.Version != *expected {
		return fmt.Errorf("dashboard '%s' is at version %d, not %d: it has been modified since you read it; read it again before updating",
			current.Metadata.Name, current.Metadata.Version, *expected)
	}
	return nil
}

// editRequest carries the common parameters of the tools modifying an existing dashboard.
type editRequest struct {
	project string
	name    string
	dryRun  bool
	include bool
}

// save validates the modified dashboard then updates it, unless this is a dry run.
func (d *dashboard) save(req editRequest, dash *v1.Dashboard, changes []string) (*mcp.CallToolResult, any, error) {
	validation, err := d.validate(dash)
	if err != nil {
		return nil, nil, err
	}
	if validation != nil && !validation.Valid {
		return invalidResult(validation)
	}
	result := writeResult{Action: "updated", DryRun: req.dryRun, Changes: changes, Warnings: warningsOf(validation)}
	if req.dryRun {
		result.Action = "would_update"
		result.Summary = dashboardutil.Summarize(dash, d.baseURL())
		return textResult(result)
	}
	saved, err := d.api(req.project).Update(dash)
	if err != nil {
		if dashboardutil.IsNotFound(err) {
			return nil, nil, notFoundError(req.name, req.project)
		}
		return nil, nil, fmt.Errorf("error updating dashboard '%s' in project '%s': %w", req.name, req.project, err)
	}
	return d.saved(result, saved, req.include)
}
