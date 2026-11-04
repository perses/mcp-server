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
	"errors"
	"fmt"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/perses/mcp-server/pkg/tools"
	"github.com/perses/perses/pkg/client/perseshttp"
	v1 "github.com/perses/perses/pkg/model/api/v1"
	"github.com/perses/spec/go/common"
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
		Description: "Create a dashboard in a project from its complete JSON definition. The Perses server " +
			"validates the dashboard when saving it; on failure its error is returned. " +
			"Returns a short summary (name, version, counts, URL), not the dashboard. " +
			"Fails if a dashboard with the same name exists; use perses_update_dashboard to change it. " +
			"Set dry_run=true to only validate. For large dashboards prefer creating a skeleton and adding panels with " +
			"perses_set_dashboard_panels.",
		InputSchema: &jsonschema.Schema{
			Type: tools.SchemaTypeObject,
			Properties: map[string]*jsonschema.Schema{
				string(tools.ProjectResource): projectSchema("Project to create the dashboard in; fills metadata.project when omitted, must match it otherwise."),
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
		dash, err := decodeDashboard(input.Dashboard, input.Project)
		if err != nil {
			return nil, nil, err
		}

		result := writeResult{Action: "created", DryRun: input.DryRun}
		if input.DryRun {
			result.Action = "would_create"
			if err := d.client.Validate().Dashboard(dash); err != nil {
				return nil, nil, err
			}
			_, err := d.api(input.Project).Get(dash.Metadata.Name)
			switch {
			case err == nil:
				return nil, nil, alreadyExistsError(dash.Metadata.Name, input.Project)
			case errors.Is(err, perseshttp.RequestNotFoundError):
			default:
				return nil, nil, fmt.Errorf("error checking whether dashboard '%s' exists in project '%s': %w", dash.Metadata.Name, input.Project, err)
			}
			result.summary = summarize(dash, d.baseURL())
			if input.IncludeDashboard {
				result.Dashboard = dash
			}
			return textResult(result)
		}

		saved, err := d.api(input.Project).Create(dash)
		switch {
		case err == nil:
		case errors.Is(err, perseshttp.ConflictError):
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

// decodeDashboard decodes a complete dashboard and checks its name, which the Perses model leaves unchecked when the
// metadata key is absent: an empty name would otherwise reach the URL of the update requests.
func decodeDashboard(raw json.RawMessage, project string) (*v1.Dashboard, error) {
	dash := &v1.Dashboard{}
	if err := json.Unmarshal(raw, dash); err != nil {
		return nil, err
	}
	if err := common.ValidateID(dash.Metadata.Name); err != nil {
		return nil, err
	}
	if err := setProject(dash, project); err != nil {
		return nil, err
	}
	return dash, nil
}

// setProject fills an empty metadata.project from the project argument, as the Perses server does when saving, so
// that the validation resolves the project variables. A different project is rejected, as the Perses server does.
// The comparison ignores case like a Perses server with the default case_sensitive: false; a case-sensitive server
// still rejects a case-only difference on the real write.
func setProject(dash *v1.Dashboard, project string) error {
	switch {
	case dash.Metadata.Project == "":
		dash.Metadata.Project = project
	case strings.EqualFold(dash.Metadata.Project, project):
	default:
		return fmt.Errorf("project parameter value '%s' does not match provided metadata value '%s'", project, dash.Metadata.Project)
	}
	return nil
}

func alreadyExistsError(name, project string) error {
	return fmt.Errorf("dashboard '%s' already exists in project '%s': use perses_update_dashboard to change it", name, project)
}

// saved completes the result with the summary of the dashboard returned by the server.
func (d *dashboard) saved(result writeResult, saved *v1.Dashboard, include bool) (*mcp.CallToolResult, any, error) {
	result.summary = summarize(saved, d.baseURL())
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
			"Validation errors are the messages of the Perses server; the response is a short summary. To change a " +
			"few panels, variables or layouts without re-sending the whole dashboard, use perses_set_dashboard_panels, " +
			"perses_set_dashboard_variables or perses_set_dashboard_layout instead.",
		InputSchema: &jsonschema.Schema{
			Type: tools.SchemaTypeObject,
			Properties: map[string]*jsonschema.Schema{
				string(tools.ProjectResource): projectSchema("Project of the dashboard; fills metadata.project when omitted, must match it otherwise."),
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
		dash, err := decodeDashboard(input.Dashboard, input.Project)
		if err != nil {
			return nil, nil, err
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
	project         string
	name            string
	expectedVersion *uint64
	dryRun          bool
	include         bool
}

// save updates the modified dashboard, which Perses validates before saving. A dry run only validates it.
func (d *dashboard) save(req editRequest, dash *v1.Dashboard, changes []string) (*mcp.CallToolResult, any, error) {
	result := writeResult{Action: "updated", DryRun: req.dryRun, Changes: changes}
	if req.dryRun {
		result.Action = "would_update"
		if err := d.client.Validate().Dashboard(dash); err != nil {
			return nil, nil, err
		}
		result.summary = summarize(dash, d.baseURL())
		if req.include {
			result.Dashboard = dash
		}
		return textResult(result)
	}
	saved, err := d.api(req.project).Update(dash)
	if err != nil {
		if errors.Is(err, perseshttp.RequestNotFoundError) {
			return nil, nil, notFoundError(req.name, req.project)
		}
		return nil, nil, fmt.Errorf("error updating dashboard '%s' in project '%s': %w", req.name, req.project, err)
	}
	return d.saved(result, saved, req.include)
}

// edit fetches the dashboard, applies the mutation and saves the result.
func (d *dashboard) edit(req editRequest, mutate func(dash *v1.Dashboard) ([]string, error)) (*mcp.CallToolResult, any, error) {
	dash, err := d.get(req.project, req.name)
	if err != nil {
		return nil, nil, err
	}
	if err := checkVersion(dash, req.expectedVersion); err != nil {
		return nil, nil, err
	}
	changes, err := mutate(dash)
	if err != nil {
		return nil, nil, err
	}
	return d.save(req, dash, changes)
}
