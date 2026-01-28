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
	"github.com/perses/mcp-server/pkg/tools/resource"
	apiClient "github.com/perses/perses/pkg/client/api/v1"
	v1 "github.com/perses/perses/pkg/model/api/v1"
)

type dashboard struct {
	client apiClient.ClientInterface
}

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

type ListDashboardsInput struct {
	Project string `json:"project" jsonschema:"Project name to list dashboards from"`
}

func (d *dashboard) List() *tools.Tool {
	tool := &mcp.Tool{
		Name:        "perses_list_dashboards",
		Description: "List dashboards for a specific project",
		Annotations: &mcp.ToolAnnotations{
			DestructiveHint: jsonschema.Ptr(false),
			IdempotentHint:  true,
			OpenWorldHint:   jsonschema.Ptr(false),
			ReadOnlyHint:    true,
			Title:           "List dashboards for a specific project in Perses",
		},
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"project": {
					Type:        "string",
					Description: "Project name",
					MinLength:   jsonschema.Ptr(1),
					MaxLength:   jsonschema.Ptr(75),
					Pattern:     "^[a-zA-Z0-9_.-]+$",
				},
			},
			Required: []string{"project"},
		},
	}

	handler := func(ctx context.Context, _ *mcp.CallToolRequest, input ListDashboardsInput) (result *mcp.CallToolResult, output any, _ error) {
		response, err := d.client.Dashboard(input.Project).List("")
		if err != nil {
			return nil, nil, fmt.Errorf("error retrieving dashboards: %w", err)
		}

		text, err := json.Marshal(response)
		if err != nil {
			return nil, nil, fmt.Errorf("error marshalling dashboards: %w", err)
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: string(text),
				},
			},
		}, nil, nil
	}

	return &tools.Tool{
		MCPTool:      tool,
		IsWriteTool:  false,
		ResourceType: tools.DashboardResource,
		RegisterWith: func(server *mcp.Server) { mcp.AddTool(server, tool, handler) },
	}
}

type GetDashboardByNameInput struct {
	Project string `json:"project" jsonschema:"Project name to retrieve the dashboard from"`
	Name    string `json:"name" jsonschema:"Dashboard name to retrieve"`
}

func (d *dashboard) Get() *tools.Tool {
	tool := &mcp.Tool{
		Name:        "perses_get_dashboard_by_name",
		Description: "Get a dashboard by name in a specific project",
		Annotations: &mcp.ToolAnnotations{
			DestructiveHint: jsonschema.Ptr(false),
			IdempotentHint:  true,
			OpenWorldHint:   jsonschema.Ptr(false),
			ReadOnlyHint:    true,
			Title:           "Gets a dashboard by name in a specific project in Perses",
		},
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"project": {
					Type:        "string",
					Description: "Project name",
					MinLength:   jsonschema.Ptr(1),
					MaxLength:   jsonschema.Ptr(75),
					Pattern:     "^[a-zA-Z0-9_.-]+$",
				},
				"name": {
					Type:        "string",
					Description: "Dashboard name",
					MinLength:   jsonschema.Ptr(1),
					MaxLength:   jsonschema.Ptr(75),
					Pattern:     "^[a-zA-Z0-9_.-]+$",
				},
			},
			Required: []string{"project", "name"},
		},
	}

	handler := func(ctx context.Context, _ *mcp.CallToolRequest, input GetDashboardByNameInput) (result *mcp.CallToolResult, output any, _ error) {
		response, err := d.client.Dashboard(input.Project).Get(input.Name)
		if err != nil {
			return nil, nil, fmt.Errorf("error retrieving dashboard '%s' in project '%s': %w", input.Name, input.Project, err)
		}
		return nil, response, nil
	}

	return &tools.Tool{
		MCPTool:      tool,
		IsWriteTool:  false,
		ResourceType: tools.DashboardResource,
		RegisterWith: func(server *mcp.Server) { mcp.AddTool(server, tool, handler) },
	}
}

type CreateDashboardInput struct {
	Project   string `json:"project" jsonschema:"Project name to create the dashboard in"`
	Dashboard string `json:"dashboard" jsonschema:"Dashboard JSON as string"`
}

func (d *dashboard) Create() *tools.Tool {
	tool := &mcp.Tool{
		Name:        "perses_create_dashboard",
		Description: "Create a new dashboard in a specific project",
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"project": {
					Type:        "string",
					Description: "Project name",
					MinLength:   jsonschema.Ptr(1),
					MaxLength:   jsonschema.Ptr(75),
					Pattern:     "^[a-zA-Z0-9_.-]+$",
				},
				"dashboard": {
					Type:        "string",
					Description: "Dashboard JSON as string",
				},
			},
			Required: []string{"project", "dashboard"},
		},
		Annotations: &mcp.ToolAnnotations{
			DestructiveHint: jsonschema.Ptr(false),
			IdempotentHint:  true,
			OpenWorldHint:   jsonschema.Ptr(false),
			ReadOnlyHint:    false,
			Title:           "Creates a new dashboard in a specific project in Perses",
		},
	}

	handler := func(ctx context.Context, _ *mcp.CallToolRequest, input CreateDashboardInput) (*mcp.CallToolResult, any, error) {
		var dashboardObj v1.Dashboard
		if err := json.Unmarshal([]byte(input.Dashboard), &dashboardObj); err != nil {
			return nil, nil, fmt.Errorf("invalid dashboard JSON: %w", err)
		}

		createdDashboard, err := d.client.Dashboard(input.Project).Create(&dashboardObj)
		if err != nil {
			return nil, nil, fmt.Errorf("error creating dashboard in project '%s': %w", input.Project, err)
		}

		dashboardJSON, err := json.Marshal(createdDashboard)
		if err != nil {
			return nil, nil, fmt.Errorf("error marshalling created dashboard: %w", err)
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: string(dashboardJSON),
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

type UpdateDashboardInput struct {
	Project   string `json:"project" jsonschema:"Project name to update the dashboard in"`
	Dashboard string `json:"dashboard" jsonschema:"Dashboard JSON as string"`
}

func (d *dashboard) Update() *tools.Tool {
	tool := &mcp.Tool{
		Name:        "perses_update_dashboard",
		Description: "Update an existing dashboard in a specific project",
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"project": {
					Type:        "string",
					Description: "Project name",
					MinLength:   jsonschema.Ptr(1),
					MaxLength:   jsonschema.Ptr(75),
					Pattern:     "^[a-zA-Z0-9_.-]+$",
				},
				"dashboard": {
					Type:        "string",
					Description: "Dashboard JSON as string",
				},
			},
			Required: []string{"project", "dashboard"},
		},
		Annotations: &mcp.ToolAnnotations{
			DestructiveHint: jsonschema.Ptr(false),
			IdempotentHint:  true,
			OpenWorldHint:   jsonschema.Ptr(false),
			ReadOnlyHint:    false,
			Title:           "Updates an existing dashboard in a specific project in Perses",
		},
	}

	handler := func(ctx context.Context, _ *mcp.CallToolRequest, input UpdateDashboardInput) (*mcp.CallToolResult, any, error) {
		var dashboardObj v1.Dashboard
		if err := json.Unmarshal([]byte(input.Dashboard), &dashboardObj); err != nil {
			return nil, nil, fmt.Errorf("invalid dashboard JSON: %w", err)
		}

		updatedDashboard, err := d.client.Dashboard(input.Project).Update(&dashboardObj)
		if err != nil {
			return nil, nil, fmt.Errorf("error updating dashboard in project '%s': %w", input.Project, err)
		}

		dashboardJSON, err := json.Marshal(updatedDashboard)
		if err != nil {
			return nil, nil, fmt.Errorf("error marshalling updated dashboard: %w", err)
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: string(dashboardJSON),
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

type DeleteDashboardInput struct {
	Project string `json:"project" jsonschema:"Project name"`
	Name    string `json:"name" jsonschema:"Dashboard name to delete"`
}

func (d *dashboard) Delete() *tools.Tool {
	tool := &mcp.Tool{
		Name:        "perses_delete_dashboard",
		Description: "Delete a dashboard from a specific project",
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"project": {
					Type:        "string",
					Description: "Project name",
					MinLength:   jsonschema.Ptr(1),
					MaxLength:   jsonschema.Ptr(75),
					Pattern:     "^[a-zA-Z0-9_.-]+$",
				},
				"name": {
					Type:        "string",
					Description: "Dashboard name",
					MinLength:   jsonschema.Ptr(1),
					MaxLength:   jsonschema.Ptr(75),
					Pattern:     "^[a-zA-Z0-9_.-]+$",
				},
			},
			Required: []string{"project", "name"},
		},
		Annotations: &mcp.ToolAnnotations{
			DestructiveHint: jsonschema.Ptr(true),
			IdempotentHint:  true,
			OpenWorldHint:   jsonschema.Ptr(false),
			ReadOnlyHint:    false,
			Title:           "Deletes a dashboard from a specific project in Perses",
		},
	}

	handler := func(ctx context.Context, _ *mcp.CallToolRequest, input DeleteDashboardInput) (*mcp.CallToolResult, any, error) {
		err := d.client.Dashboard(input.Project).Delete(input.Name)
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
