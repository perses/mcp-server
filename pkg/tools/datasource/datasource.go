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
	"github.com/perses/mcp-server/pkg/tools/internal/proxy"
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
		d.Query(),
		d.Create(),
		d.Update(),
		d.Delete(),
	}
}

type QueryProjectDatasourceInput struct {
	Project        string            `json:"project" jsonschema:"Project name"`
	DatasourceName string            `json:"datasource_name" jsonschema:"Datasource name"`
	Method         string            `json:"method,omitempty" jsonschema:"HTTP method (default GET)"`
	Path           string            `json:"path,omitempty" jsonschema:"Datasource endpoint path (e.g. /api/v1/query_range)"`
	QueryParams    map[string]string `json:"query_params,omitempty" jsonschema:"Query string parameters"`
	Body           string            `json:"body,omitempty" jsonschema:"Raw request body (JSON or form payload)"`
	Headers        map[string]string `json:"headers,omitempty" jsonschema:"Optional additional request headers"`
}

func (d *datasource) Query() *tools.Tool {
	tool := &mcp.Tool{
		Name:        "perses_query_project_datasource",
		Description: "Query a project datasource through Perses proxy",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Queries a project datasource through Perses proxy",
			ReadOnlyHint:    true,
			DestructiveHint: new(false),
			IdempotentHint:  true,
			OpenWorldHint:   new(false),
		},
		InputSchema: &jsonschema.Schema{
			Type: tools.SchemaTypeObject,
			Properties: map[string]*jsonschema.Schema{
				string(tools.ProjectResource): {
					Type:        tools.SchemaTypeString,
					Description: "Project name",
					MinLength:   new(1),
					MaxLength:   new(75),
					Pattern:     tools.PatternResourceName,
				},
				"datasource_name": {
					Type:        tools.SchemaTypeString,
					Description: "Datasource name",
					MinLength:   new(1),
					MaxLength:   new(75),
					Pattern:     tools.PatternResourceName,
				},
				"method": {
					Type:        tools.SchemaTypeString,
					Description: "HTTP method (default GET)",
					Enum:        []any{"GET", "POST"},
				},
				"path": {
					Type:        tools.SchemaTypeString,
					Description: "Datasource endpoint path",
				},
				"query_params": {
					Type:        tools.SchemaTypeObject,
					Description: "Query string parameters",
					AdditionalProperties: &jsonschema.Schema{
						Type: tools.SchemaTypeString,
					},
				},
				"body": {
					Type:        tools.SchemaTypeString,
					Description: "Raw request body (JSON or form payload)",
				},
				"headers": {
					Type:        tools.SchemaTypeObject,
					Description: "Optional additional request headers",
					AdditionalProperties: &jsonschema.Schema{
						Type: tools.SchemaTypeString,
					},
				},
			},
			Required: []string{"project", "datasource_name"},
		},
	}

	handler := func(ctx context.Context, _ *mcp.CallToolRequest, input QueryProjectDatasourceInput) (*mcp.CallToolResult, any, error) { //nolint:unparam
		helper := proxy.New(d.client)

		sharedInput := proxy.ProxyQuery{
			Method:      input.Method,
			Path:        proxy.ProjectDatasourceProxyPath(input.Project, input.DatasourceName, input.Path),
			QueryParams: input.QueryParams,
			Body:        input.Body,
			Headers:     input.Headers,
		}

		result, err := helper.Do(ctx, sharedInput)
		if err != nil {
			return nil, nil, err
		}

		resultJSON, err := json.Marshal(result)
		if err != nil {
			return nil, nil, fmt.Errorf("error marshalling datasource query result: %w", err)
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: string(resultJSON)},
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
			DestructiveHint: new(false),
			IdempotentHint:  true,
			OpenWorldHint:   new(false),
		},
		InputSchema: &jsonschema.Schema{
			Type: tools.SchemaTypeObject,
			Properties: map[string]*jsonschema.Schema{
				string(tools.ProjectResource): {
					Type:        tools.SchemaTypeString,
					Description: "Project name",
					MinLength:   new(1),
					MaxLength:   new(75),
					Pattern:     tools.PatternResourceName,
				},
			},
			Required: []string{string(tools.ProjectResource)},
		},
	}

	handler := func(_ context.Context, _ *mcp.CallToolRequest, input ListProjectDatasourcesInput) (*mcp.CallToolResult, any, error) { //nolint:unparam
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
			DestructiveHint: new(false),
			IdempotentHint:  true,
			OpenWorldHint:   new(false),
		},
		InputSchema: &jsonschema.Schema{
			Type: tools.SchemaTypeObject,
			Properties: map[string]*jsonschema.Schema{
				string(tools.ProjectResource): {
					Type:        tools.SchemaTypeString,
					Description: "Project name",
					MinLength:   new(1),
					MaxLength:   new(75),
					Pattern:     tools.PatternResourceName,
				},
				tools.ResourceName: {
					Type:        tools.SchemaTypeString,
					Description: "Datasource name",
					MinLength:   new(1),
					MaxLength:   new(75),
					Pattern:     tools.PatternResourceName,
				},
			},
			Required: []string{string(tools.ProjectResource), tools.ResourceName},
		},
	}

	handler := func(_ context.Context, _ *mcp.CallToolRequest, input GetProjectDatasourceByNameInput) (*mcp.CallToolResult, any, error) { //nolint:unparam
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
			Type: tools.SchemaTypeObject,
			Properties: map[string]*jsonschema.Schema{
				string(tools.ProjectResource): {
					Type:        tools.SchemaTypeString,
					Description: "Project name",
					MinLength:   new(1),
					MaxLength:   new(75),
					Pattern:     tools.PatternResourceName,
				},
				string(tools.DatasourceResource): {
					Type:        tools.SchemaTypeString,
					Description: "Datasource JSON as string",
				},
			},
			Required: []string{string(tools.ProjectResource), string(tools.DatasourceResource)},
		},
		Annotations: &mcp.ToolAnnotations{
			DestructiveHint: new(false),
			IdempotentHint:  true,
			OpenWorldHint:   new(false),
			ReadOnlyHint:    false,
			Title:           "Creates a new datasource in a specific project in Perses",
		},
	}

	handler := func(_ context.Context, _ *mcp.CallToolRequest, input CreateDatasourceInput) (*mcp.CallToolResult, any, error) { //nolint:unparam
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
			Type: tools.SchemaTypeObject,
			Properties: map[string]*jsonschema.Schema{
				string(tools.ProjectResource): {
					Type:        tools.SchemaTypeString,
					Description: "Project name",
					MinLength:   new(1),
					MaxLength:   new(75),
					Pattern:     tools.PatternResourceName,
				},
				string(tools.DatasourceResource): {
					Type:        tools.SchemaTypeString,
					Description: "Datasource JSON as string",
				},
			},
			Required: []string{string(tools.ProjectResource), string(tools.DatasourceResource)},
		},
		Annotations: &mcp.ToolAnnotations{
			DestructiveHint: new(false),
			IdempotentHint:  true,
			OpenWorldHint:   new(false),
			ReadOnlyHint:    false,
			Title:           "Updates an existing datasource in a specific project in Perses",
		},
	}

	handler := func(_ context.Context, _ *mcp.CallToolRequest, input UpdateDatasourceInput) (*mcp.CallToolResult, any, error) { //nolint:unparam
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
			Type: tools.SchemaTypeObject,
			Properties: map[string]*jsonschema.Schema{
				string(tools.ProjectResource): {
					Type:        tools.SchemaTypeString,
					Description: "Project name",
					MinLength:   new(1),
					MaxLength:   new(75),
					Pattern:     tools.PatternResourceName,
				},
				tools.ResourceName: {
					Type:        tools.SchemaTypeString,
					Description: "Datasource name",
					MinLength:   new(1),
					MaxLength:   new(75),
					Pattern:     tools.PatternResourceName,
				},
			},
			Required: []string{string(tools.ProjectResource), tools.ResourceName},
		},
		Annotations: &mcp.ToolAnnotations{
			DestructiveHint: new(true),
			IdempotentHint:  true,
			OpenWorldHint:   new(false),
			ReadOnlyHint:    false,
			Title:           "Deletes a datasource from a specific project in Perses",
		},
	}

	handler := func(_ context.Context, _ *mcp.CallToolRequest, input DeleteDatasourceInput) (*mcp.CallToolResult, any, error) { //nolint:unparam
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
