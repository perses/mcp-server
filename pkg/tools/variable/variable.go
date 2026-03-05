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

package variable

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
	"github.com/perses/perses/pkg/model/api/v1/variable"
)

type projectVariable struct {
	client apiClient.ClientInterface
}

func New(client apiClient.ClientInterface) resource.Resource {
	return &projectVariable{
		client: client,
	}
}

func (v *projectVariable) GetTools() []*tools.Tool {
	return []*tools.Tool{
		v.List(),
		v.Get(),
		v.Create(),
		v.Update(),
		v.Delete(),
	}
}

type ListProjectVariablesInput struct {
	Project string `json:"project" jsonschema:"Project name"`
}

func (v *projectVariable) List() *tools.Tool {
	tool := &mcp.Tool{
		Name:        "perses_list_project_variables",
		Description: "List variables for a specific project",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Lists variables for a specific project in Perses",
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

	handler := func(_ context.Context, _ *mcp.CallToolRequest, input ListProjectVariablesInput) (*mcp.CallToolResult, any, error) { //nolint: unparam
		variables, err := v.client.Variable(input.Project).List("")
		if err != nil {
			return nil, nil, fmt.Errorf("error retrieving variables in project '%s': %w", input.Project, err)
		}

		variablesJSON, err := json.Marshal(variables)
		if err != nil {
			return nil, nil, fmt.Errorf("error marshalling variables: %w", err)
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: string(variablesJSON),
				},
			},
		}, nil, nil
	}

	return &tools.Tool{
		MCPTool:      tool,
		IsWriteTool:  false,
		ResourceType: tools.VariableResource,
		RegisterWith: func(server *mcp.Server) { mcp.AddTool(server, tool, handler) },
	}
}

type GetProjectVariableByNameInput struct {
	Project string `json:"project" jsonschema:"Project name"`
	Name    string `json:"name" jsonschema:"Variable name"`
}

func (v *projectVariable) Get() *tools.Tool {
	tool := &mcp.Tool{
		Name:        "perses_get_project_variable_by_name",
		Description: "Get a variable by name in a specific project",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Gets a variable by name in a specific project in Perses",
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
					Description: "Variable name",
					MinLength:   jsonschema.Ptr(1),
					MaxLength:   jsonschema.Ptr(75),
					Pattern:     "^[a-zA-Z0-9_.-]+$",
				},
			},
			Required: []string{"project", "name"},
		},
	}

	handler := func(_ context.Context, _ *mcp.CallToolRequest, input GetProjectVariableByNameInput) (*mcp.CallToolResult, any, error) { //nolint: unparam
		projectVar, err := v.client.Variable(input.Project).Get(input.Name)
		if err != nil {
			return nil, nil, fmt.Errorf("error retrieving variable '%s' in project '%s': %w", input.Name, input.Project, err)
		}

		variableJSON, err := json.Marshal(projectVar)
		if err != nil {
			return nil, nil, fmt.Errorf("error marshalling variable: %w", err)
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: string(variableJSON),
				},
			},
		}, nil, nil
	}

	return &tools.Tool{
		MCPTool:      tool,
		IsWriteTool:  false,
		ResourceType: tools.VariableResource,
		RegisterWith: func(server *mcp.Server) { mcp.AddTool(server, tool, handler) },
	}
}

type CreateProjectVariableInput struct {
	Name    string `json:"name" jsonschema:"Variable name"`
	Project string `json:"project" jsonschema:"Project name"`
}

func (v *projectVariable) Create() *tools.Tool {
	tool := &mcp.Tool{
		Name:        "perses_create_project_variable",
		Description: "Create a project level variable",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Creates a project level variable in Perses",
			ReadOnlyHint:    false,
			DestructiveHint: jsonschema.Ptr(false),
			IdempotentHint:  true,
			OpenWorldHint:   jsonschema.Ptr(false),
		},
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"name": {
					Type:        "string",
					Description: "Variable name",
					MinLength:   jsonschema.Ptr(1),
					MaxLength:   jsonschema.Ptr(75),
					Pattern:     "^[a-zA-Z0-9_.-]+$",
				},
				"project": {
					Type:        "string",
					Description: "Project name",
					MinLength:   jsonschema.Ptr(1),
					MaxLength:   jsonschema.Ptr(75),
					Pattern:     "^[a-zA-Z0-9_.-]+$",
				},
			},
			Required: []string{"name", "project"},
		},
	}

	handler := func(_ context.Context, _ *mcp.CallToolRequest, input CreateProjectVariableInput) (*mcp.CallToolResult, any, error) { //nolint: unparam
		projectVar := &v1.Variable{
			Kind: "Variable",
			Metadata: v1.ProjectMetadata{
				Metadata: v1.Metadata{
					Name: input.Name,
				},
				ProjectMetadataWrapper: v1.ProjectMetadataWrapper{
					Project: input.Project,
				},
			},
			Spec: v1.VariableSpec{
				Kind: variable.KindText,
				Spec: &variable.TextSpec{
					Value: input.Name,
				},
			},
		}

		result, err := v.client.Variable(input.Project).Create(projectVar)
		if err != nil {
			return nil, nil, fmt.Errorf("error creating variable '%s' in project '%s': %w", input.Name, input.Project, err)
		}

		resultJSON, err := json.Marshal(result)
		if err != nil {
			return nil, nil, fmt.Errorf("error marshalling variable result: %w", err)
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: string(resultJSON),
				},
			},
		}, nil, nil
	}

	return &tools.Tool{
		MCPTool:      tool,
		IsWriteTool:  true,
		ResourceType: tools.VariableResource,
		RegisterWith: func(server *mcp.Server) { mcp.AddTool(server, tool, handler) },
	}
}

type UpdateProjectVariableInput struct {
	Name    string `json:"name" jsonschema:"Variable name"`
	Project string `json:"project" jsonschema:"Project name"`
	Value   string `json:"value" jsonschema:"Variable value (for TextVariable)"`
}

func (v *projectVariable) Update() *tools.Tool {
	tool := &mcp.Tool{
		Name:        "perses_update_project_variable",
		Description: "Update an existing project level variable (TextVariable type)",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Updates a project level variable in Perses",
			ReadOnlyHint:    false,
			DestructiveHint: jsonschema.Ptr(false),
			IdempotentHint:  true,
			OpenWorldHint:   jsonschema.Ptr(false),
		},
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"name": {
					Type:        "string",
					Description: "Variable name",
					MinLength:   jsonschema.Ptr(1),
					MaxLength:   jsonschema.Ptr(75),
					Pattern:     "^[a-zA-Z0-9_.-]+$",
				},
				"project": {
					Type:        "string",
					Description: "Project name",
					MinLength:   jsonschema.Ptr(1),
					MaxLength:   jsonschema.Ptr(75),
					Pattern:     "^[a-zA-Z0-9_.-]+$",
				},
				"value": {
					Type:        "string",
					Description: "Variable value",
				},
			},
			Required: []string{"name", "project", "value"},
		},
	}

	handler := func(_ context.Context, _ *mcp.CallToolRequest, input UpdateProjectVariableInput) (*mcp.CallToolResult, any, error) { //nolint: unparam
		projectVar := &v1.Variable{
			Kind: v1.KindVariable,
			Metadata: v1.ProjectMetadata{
				Metadata: v1.Metadata{
					Name: input.Name,
				},
				ProjectMetadataWrapper: v1.ProjectMetadataWrapper{
					Project: input.Project,
				},
			},
			Spec: v1.VariableSpec{
				Kind: variable.KindText,
				Spec: &variable.TextSpec{
					Value: input.Value,
				},
			},
		}

		result, err := v.client.Variable(input.Project).Update(projectVar)
		if err != nil {
			return nil, nil, fmt.Errorf("error updating variable '%s' in project '%s': %w", input.Name, input.Project, err)
		}

		resultJSON, err := json.Marshal(result)
		if err != nil {
			return nil, nil, fmt.Errorf("error marshalling updated variable: %w", err)
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: string(resultJSON),
				},
			},
		}, nil, nil
	}

	return &tools.Tool{
		MCPTool:      tool,
		IsWriteTool:  true,
		ResourceType: tools.VariableResource,
		RegisterWith: func(server *mcp.Server) { mcp.AddTool(server, tool, handler) },
	}
}

type DeleteProjectVariableInput struct {
	Name    string `json:"name" jsonschema:"Variable name to delete"`
	Project string `json:"project" jsonschema:"Project name"`
}

func (v *projectVariable) Delete() *tools.Tool {
	tool := &mcp.Tool{
		Name:        "perses_delete_project_variable",
		Description: "Delete a project level variable",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Deletes a project level variable in Perses",
			ReadOnlyHint:    false,
			DestructiveHint: jsonschema.Ptr(true),
			IdempotentHint:  true,
			OpenWorldHint:   jsonschema.Ptr(false),
		},
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"name": {
					Type:        "string",
					Description: "Variable name",
					MinLength:   jsonschema.Ptr(1),
					MaxLength:   jsonschema.Ptr(75),
					Pattern:     "^[a-zA-Z0-9_.-]+$",
				},
				"project": {
					Type:        "string",
					Description: "Project name",
					MinLength:   jsonschema.Ptr(1),
					MaxLength:   jsonschema.Ptr(75),
					Pattern:     "^[a-zA-Z0-9_.-]+$",
				},
			},
			Required: []string{"name", "project"},
		},
	}

	handler := func(_ context.Context, _ *mcp.CallToolRequest, input DeleteProjectVariableInput) (*mcp.CallToolResult, any, error) { //nolint: unparam
		err := v.client.Variable(input.Project).Delete(input.Name)
		if err != nil {
			return nil, nil, fmt.Errorf("error deleting variable '%s' in project '%s': %w", input.Name, input.Project, err)
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: fmt.Sprintf("Variable '%s' deleted successfully from project '%s'", input.Name, input.Project),
				},
			},
		}, nil, nil
	}

	return &tools.Tool{
		MCPTool:      tool,
		IsWriteTool:  true,
		ResourceType: tools.VariableResource,
		RegisterWith: func(server *mcp.Server) { mcp.AddTool(server, tool, handler) },
	}
}
