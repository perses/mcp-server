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

package datasource

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

type datasource struct {
	client apiClient.ClientInterface
}

func New(client apiClient.ClientInterface) resource.Resource {
	return &datasource{
		client: client,
	}
}

func (d *datasource) GetTools() []*tools.Tool {
	return []*tools.Tool{
		d.List(),
		d.Get(),
		d.Create(),
		d.Update(),
		d.Delete(),
	}
}

type ListProjectDatasourcesInput struct {
	Project string `json:"project" jsonschema:"Project name"`
}

func (d *datasource) List() *tools.Tool {
	tool := &mcp.Tool{
		Name:        "perses_list_project_datasources",
		Description: "List Datasources for a specific project",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Lists datasources for a specific project in Perses",
			ReadOnlyHint:    true,
			DestructiveHint: jsonschema.Ptr(false),
			IdempotentHint:  true,
			OpenWorldHint:   jsonschema.Ptr(false),
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

	handler := func(_ context.Context, _ *mcp.CallToolRequest, input ListProjectDatasourcesInput) (*mcp.CallToolResult, any, error) { //nolint: unparam
		datasources, err := d.client.Datasource(input.Project).List("")
		if err != nil {
			return nil, nil, fmt.Errorf("error retrieving datasources in project '%s': %w", input.Project, err)
		}

		datasourcesJSON, err := json.Marshal(datasources)
		if err != nil {
			return nil, nil, fmt.Errorf("error marshalling datasources: %w", err)
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: string(datasourcesJSON),
				},
			},
		}, nil, nil
	}

	return &tools.Tool{
		MCPTool:      tool,
		IsWriteTool:  false,
		ResourceType: tools.DatasourceResource,
		RegisterWith: func(server *mcp.Server) { mcp.AddTool(server, tool, handler) },
	}
}

type GetProjectDatasourceByNameInput struct {
	Project string `json:"project" jsonschema:"Project name"`
	Name    string `json:"name" jsonschema:"Datasource name"`
}

func (d *datasource) Get() *tools.Tool {
	tool := &mcp.Tool{
		Name:        "perses_get_project_datasource_by_name",
		Description: "Get a datasource by name in a specific project",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Gets a datasource by name in a specific project in Perses",
			ReadOnlyHint:    true,
			DestructiveHint: jsonschema.Ptr(false),
			IdempotentHint:  true,
			OpenWorldHint:   jsonschema.Ptr(false),
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
					Description: "Datasource name",
					MinLength:   jsonschema.Ptr(1),
					MaxLength:   jsonschema.Ptr(75),
					Pattern:     "^[a-zA-Z0-9_.-]+$",
				},
			},
			Required: []string{"project", "name"},
		},
	}

	handler := func(_ context.Context, _ *mcp.CallToolRequest, input GetProjectDatasourceByNameInput) (*mcp.CallToolResult, any, error) { //nolint: unparam
		datasource, err := d.client.Datasource(input.Project).Get(input.Name)
		if err != nil {
			return nil, nil, fmt.Errorf("error retrieving datasource '%s' in project '%s': %w", input.Name, input.Project, err)
		}

		datasourceJSON, err := json.Marshal(datasource)
		if err != nil {
			return nil, nil, fmt.Errorf("error marshalling datasource: %w", err)
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: string(datasourceJSON),
				},
			},
		}, nil, nil
	}

	return &tools.Tool{
		MCPTool:      tool,
		IsWriteTool:  false,
		ResourceType: tools.DatasourceResource,
		RegisterWith: func(server *mcp.Server) { mcp.AddTool(server, tool, handler) },
	}
}

type CreateDatasourceInput struct {
	Project    string `json:"project" jsonschema:"Project name to create the datasource in"`
	Datasource string `json:"datasource" jsonschema:"Datasource JSON as string"`
}

func (d *datasource) Create() *tools.Tool {
	tool := &mcp.Tool{
		Name:        "perses_create_project_datasource",
		Description: "Create a new datasource in a specific project",
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
				"datasource": {
					Type:        "string",
					Description: "Datasource JSON as string",
				},
			},
			Required: []string{"project", "datasource"},
		},
		Annotations: &mcp.ToolAnnotations{
			DestructiveHint: jsonschema.Ptr(false),
			IdempotentHint:  true,
			OpenWorldHint:   jsonschema.Ptr(false),
			ReadOnlyHint:    false,
			Title:           "Creates a new datasource in a specific project in Perses",
		},
	}

	handler := func(_ context.Context, _ *mcp.CallToolRequest, input CreateDatasourceInput) (*mcp.CallToolResult, any, error) { //nolint: unparam
		var datasourceObj v1.Datasource
		if err := json.Unmarshal([]byte(input.Datasource), &datasourceObj); err != nil {
			return nil, nil, fmt.Errorf("invalid datasource JSON: %w", err)
		}

		createdDatasource, err := d.client.Datasource(input.Project).Create(&datasourceObj)
		if err != nil {
			return nil, nil, fmt.Errorf("error creating datasource in project '%s': %w", input.Project, err)
		}

		datasourceJSON, err := json.Marshal(createdDatasource)
		if err != nil {
			return nil, nil, fmt.Errorf("error marshalling created datasource: %w", err)
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: string(datasourceJSON),
				},
			},
		}, nil, nil
	}

	return &tools.Tool{
		MCPTool:      tool,
		IsWriteTool:  true,
		ResourceType: tools.DatasourceResource,
		RegisterWith: func(server *mcp.Server) { mcp.AddTool(server, tool, handler) },
	}
}

type UpdateDatasourceInput struct {
	Project    string `json:"project" jsonschema:"Project name to update the datasource in"`
	Datasource string `json:"datasource" jsonschema:"Datasource JSON as string"`
}

func (d *datasource) Update() *tools.Tool {
	tool := &mcp.Tool{
		Name:        "perses_update_project_datasource",
		Description: "Update an existing datasource in a specific project",
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
				"datasource": {
					Type:        "string",
					Description: "Datasource JSON as string",
				},
			},
			Required: []string{"project", "datasource"},
		},
		Annotations: &mcp.ToolAnnotations{
			DestructiveHint: jsonschema.Ptr(false),
			IdempotentHint:  true,
			OpenWorldHint:   jsonschema.Ptr(false),
			ReadOnlyHint:    false,
			Title:           "Updates an existing datasource in a specific project in Perses",
		},
	}

	handler := func(_ context.Context, _ *mcp.CallToolRequest, input UpdateDatasourceInput) (*mcp.CallToolResult, any, error) { //nolint: unparam
		var datasourceObj v1.Datasource
		if err := json.Unmarshal([]byte(input.Datasource), &datasourceObj); err != nil {
			return nil, nil, fmt.Errorf("invalid datasource JSON: %w", err)
		}

		updatedDatasource, err := d.client.Datasource(input.Project).Update(&datasourceObj)
		if err != nil {
			return nil, nil, fmt.Errorf("error updating datasource in project '%s': %w", input.Project, err)
		}

		datasourceJSON, err := json.Marshal(updatedDatasource)
		if err != nil {
			return nil, nil, fmt.Errorf("error marshalling updated datasource: %w", err)
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: string(datasourceJSON),
				},
			},
		}, nil, nil
	}

	return &tools.Tool{
		MCPTool:      tool,
		IsWriteTool:  true,
		ResourceType: tools.DatasourceResource,
		RegisterWith: func(server *mcp.Server) { mcp.AddTool(server, tool, handler) },
	}
}

type DeleteDatasourceInput struct {
	Project string `json:"project" jsonschema:"Project name"`
	Name    string `json:"name" jsonschema:"Datasource name to delete"`
}

func (d *datasource) Delete() *tools.Tool {
	tool := &mcp.Tool{
		Name:        "perses_delete_project_datasource",
		Description: "Delete a datasource from a specific project",
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
					Description: "Datasource name",
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
			Title:           "Deletes a datasource from a specific project in Perses",
		},
	}

	handler := func(_ context.Context, _ *mcp.CallToolRequest, input DeleteDatasourceInput) (*mcp.CallToolResult, any, error) { //nolint: unparam
		err := d.client.Datasource(input.Project).Delete(input.Name)
		if err != nil {
			return nil, nil, fmt.Errorf("error deleting datasource '%s' in project '%s': %w", input.Name, input.Project, err)
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: fmt.Sprintf("Datasource '%s' deleted successfully from project '%s'", input.Name, input.Project),
				},
			},
		}, nil, nil
	}

	return &tools.Tool{
		MCPTool:      tool,
		IsWriteTool:  true,
		ResourceType: tools.DatasourceResource,
		RegisterWith: func(server *mcp.Server) { mcp.AddTool(server, tool, handler) },
	}
}
