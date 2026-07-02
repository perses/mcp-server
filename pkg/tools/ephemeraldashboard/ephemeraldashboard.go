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

package ephemeraldashboard

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

type ephemeraldashboard struct {
	client apiClient.ClientInterface
}

func New(client apiClient.ClientInterface) resource.Resource {
	return &ephemeraldashboard{
		client: client,
	}
}

func (ed *ephemeraldashboard) GetTools() []*tools.Tool {
	return []*tools.Tool{
		ed.List(),
		ed.Get(),
		ed.Create(),
		ed.Update(),
		ed.Delete(),
	}
}

type ListEphemeralDashboardsInput struct {
	Project string `json:"project" jsonschema:"Project name to list ephemeral dashboards from"`
}

func (ed *ephemeraldashboard) List() *tools.Tool {
	tool := &mcp.Tool{
		Name:        "perses_list_ephemeral_dashboards",
		Description: "List ephemeral dashboards for a specific project",
		Annotations: &mcp.ToolAnnotations{
			DestructiveHint: jsonschema.Ptr(false),
			IdempotentHint:  true,
			OpenWorldHint:   jsonschema.Ptr(false),
			ReadOnlyHint:    true,
			Title:           "List ephemeral dashboards for a specific project in Perses",
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

	handler := func(_ context.Context, _ *mcp.CallToolRequest, input ListEphemeralDashboardsInput) (*mcp.CallToolResult, any, error) { //nolint:unparam
		response, err := ed.client.EphemeralDashboard(input.Project).List("")
		if err != nil {
			return nil, nil, fmt.Errorf("error retrieving ephemeral dashboards: %w", err)
		}

		text, err := json.Marshal(response)
		if err != nil {
			return nil, nil, fmt.Errorf("error marshalling ephemeral dashboards: %w", err)
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
		ResourceType: tools.EphemeralDashboardResource,
		RegisterWith: func(server *mcp.Server) { mcp.AddTool(server, tool, handler) },
	}
}

type GetEphemeralDashboardByNameInput struct {
	Project string `json:"project" jsonschema:"Project name to retrieve the ephemeral dashboard from"`
	Name    string `json:"name" jsonschema:"Ephemeral dashboard name to retrieve"`
}

func (ed *ephemeraldashboard) Get() *tools.Tool {
	tool := &mcp.Tool{
		Name:        "perses_get_ephemeral_dashboard_by_name",
		Description: "Get an ephemeral dashboard by name in a specific project",
		Annotations: &mcp.ToolAnnotations{
			DestructiveHint: jsonschema.Ptr(false),
			IdempotentHint:  true,
			OpenWorldHint:   jsonschema.Ptr(false),
			ReadOnlyHint:    true,
			Title:           "Gets an ephemeral dashboard by name in a specific project in Perses",
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

	handler := func(_ context.Context, _ *mcp.CallToolRequest, input GetEphemeralDashboardByNameInput) (*mcp.CallToolResult, any, error) { //nolint:unparam
		response, err := ed.client.EphemeralDashboard(input.Project).Get(input.Name)
		if err != nil {
			return nil, nil, fmt.Errorf("error retrieving ephemeral dashboard '%s' in project '%s': %w", input.Name, input.Project, err)
		}
		return nil, response, nil
	}

	return &tools.Tool{
		MCPTool:      tool,
		IsWriteTool:  false,
		ResourceType: tools.EphemeralDashboardResource,
		RegisterWith: func(server *mcp.Server) { mcp.AddTool(server, tool, handler) },
	}
}

type CreateEphemeralDashboardInput struct {
	Project   string `json:"project" jsonschema:"Project name to create the ephemeral dashboard in"`
	Dashboard string `json:"dashboard" jsonschema:"Ephemeral dashboard JSON as string"`
}

func (ed *ephemeraldashboard) Create() *tools.Tool {
	tool := &mcp.Tool{
		Name:        "perses_create_ephemeral_dashboard",
		Description: "Create a new ephemeral dashboard in a specific project",
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
					Description: "Ephemeral dashboard JSON as string",
				},
			},
			Required: []string{"project", "dashboard"},
		},
		Annotations: &mcp.ToolAnnotations{
			DestructiveHint: jsonschema.Ptr(false),
			IdempotentHint:  true,
			OpenWorldHint:   jsonschema.Ptr(false),
			ReadOnlyHint:    false,
			Title:           "Creates a new ephemeral dashboard in a specific project in Perses",
		},
	}

	handler := func(_ context.Context, _ *mcp.CallToolRequest, input CreateEphemeralDashboardInput) (*mcp.CallToolResult, any, error) { //nolint:unparam
		var dashboardObj v1.EphemeralDashboard
		if err := json.Unmarshal([]byte(input.Dashboard), &dashboardObj); err != nil {
			return nil, nil, fmt.Errorf("invalid dashboard JSON: %w", err)
		}

		createdDashboard, err := ed.client.EphemeralDashboard(input.Project).Create(&dashboardObj)
		if err != nil {
			return nil, nil, fmt.Errorf("error creating ephemeral dashboard in project '%s': %w", input.Project, err)
		}

		dashboardJSON, err := json.Marshal(createdDashboard)
		if err != nil {
			return nil, nil, fmt.Errorf("error marshalling created ephemeral dashboard: %w", err)
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
		ResourceType: tools.EphemeralDashboardResource,
		RegisterWith: func(server *mcp.Server) { mcp.AddTool(server, tool, handler) },
	}
}

type UpdateEphemeralDashboardInput struct {
	Project   string `json:"project" jsonschema:"Project name to update the ephemeral dashboard in"`
	Dashboard string `json:"dashboard" jsonschema:"Ephemeral dashboard JSON as string"`
}

func (ed *ephemeraldashboard) Update() *tools.Tool {
	tool := &mcp.Tool{
		Name:        "perses_update_ephemeral_dashboard",
		Description: "Update an existing ephemeral dashboard in a specific project",
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
					Description: "Ephemeral dashboard JSON as string",
				},
			},
			Required: []string{"project", "dashboard"},
		},
		Annotations: &mcp.ToolAnnotations{
			DestructiveHint: jsonschema.Ptr(false),
			IdempotentHint:  true,
			OpenWorldHint:   jsonschema.Ptr(false),
			ReadOnlyHint:    false,
			Title:           "Updates an existing ephemeral dashboard in a specific project in Perses",
		},
	}

	handler := func(_ context.Context, _ *mcp.CallToolRequest, input UpdateEphemeralDashboardInput) (*mcp.CallToolResult, any, error) { //nolint:unparam
		var dashboardObj v1.EphemeralDashboard
		if err := json.Unmarshal([]byte(input.Dashboard), &dashboardObj); err != nil {
			return nil, nil, fmt.Errorf("invalid ephemeral dashboard JSON: %w", err)
		}

		updatedDashboard, err := ed.client.EphemeralDashboard(input.Project).Update(&dashboardObj)
		if err != nil {
			return nil, nil, fmt.Errorf("error updating ephemeral dashboard in project '%s': %w", input.Project, err)
		}

		dashboardJSON, err := json.Marshal(updatedDashboard)
		if err != nil {
			return nil, nil, fmt.Errorf("error marshalling updated ephemeral dashboard: %w", err)
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
		ResourceType: tools.EphemeralDashboardResource,
		RegisterWith: func(server *mcp.Server) { mcp.AddTool(server, tool, handler) },
	}
}

type DeleteEphemeralDashboardInput struct {
	Project string `json:"project" jsonschema:"Project name"`
	Name    string `json:"name" jsonschema:"Ephemeral dashboard name to delete"`
}

func (ed *ephemeraldashboard) Delete() *tools.Tool {
	tool := &mcp.Tool{
		Name:        "perses_delete_ephemeral_dashboard",
		Description: "Delete an ephemeral dashboard from a specific project",
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
					Description: "Ephemeral dashboard name",
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
			Title:           "Deletes an ephemeral dashboard from a specific project in Perses",
		},
	}

	handler := func(_ context.Context, _ *mcp.CallToolRequest, input DeleteEphemeralDashboardInput) (*mcp.CallToolResult, any, error) { //nolint:unparam
		err := ed.client.EphemeralDashboard(input.Project).Delete(input.Name)
		if err != nil {
			return nil, nil, fmt.Errorf("error deleting ephemeral dashboard '%s' in project '%s': %w", input.Name, input.Project, err)
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: fmt.Sprintf("Ephemeral dashboard '%s' deleted successfully from project '%s'", input.Name, input.Project),
				},
			},
		}, nil, nil
	}

	return &tools.Tool{
		MCPTool:      tool,
		IsWriteTool:  true,
		ResourceType: tools.EphemeralDashboardResource,
		RegisterWith: func(server *mcp.Server) { mcp.AddTool(server, tool, handler) },
	}
}
