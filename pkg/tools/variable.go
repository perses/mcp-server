// Copyright 2025 The Perses Authors
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

package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	apiClient "github.com/perses/perses/pkg/client/api/v1"
	v1 "github.com/perses/perses/pkg/model/api/v1"
	"github.com/perses/perses/pkg/model/api/v1/variable"
)

type VariableInterface interface {
	List() *Tool
	Get() *Tool
	Create() *Tool
	GetTools() []*Tool
}

type projectVariable struct {
	VariableInterface
	client apiClient.ClientInterface
}

func newVariable(client apiClient.ClientInterface) VariableInterface {
	return &projectVariable{
		client: client,
	}
}

func (v *projectVariable) GetTools() []*Tool {
	return []*Tool{
		v.List(),
		v.Get(),
		v.Create(),
	}
}

type ListProjectVariablesInput struct {
	Project string `json:"project" jsonschema:"Project name"`
}

func (v *projectVariable) List() *Tool {
	tool := mcp.Tool{
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

	handler := func(ctx context.Context, _ *mcp.CallToolRequest, input ListProjectVariablesInput) (*mcp.CallToolResult, any, error) {
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

	return &Tool{
		MCPTool:     &tool,
		RegisterWith: func(server *mcp.Server) { mcp.AddTool(server, &tool, handler) },
		IsWriteTool: false,
	}
}

type GetProjectVariableByNameInput struct {
	Project string `json:"project" jsonschema:"Project name"`
	Name    string `json:"name" jsonschema:"Variable name"`
}

func (v *projectVariable) Get() *Tool {
	tool := mcp.Tool{
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

	handler := func(ctx context.Context, _ *mcp.CallToolRequest, input GetProjectVariableByNameInput) (*mcp.CallToolResult, any, error) {
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

	return &Tool{
		MCPTool:     &tool,
		RegisterWith: func(server *mcp.Server) { mcp.AddTool(server, &tool, handler) },
		IsWriteTool: false,
	}
}

type CreateProjectVariableInput struct {
	Name    string `json:"name" jsonschema:"Variable name"`
	Project string `json:"project" jsonschema:"Project name"`
}

func (v *projectVariable) Create() *Tool {
	tool := mcp.Tool{
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

	handler := func(ctx context.Context, _ *mcp.CallToolRequest, input CreateProjectVariableInput) (*mcp.CallToolResult, any, error) {
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

	return &Tool{
		MCPTool:     &tool,
		RegisterWith: func(server *mcp.Server) { mcp.AddTool(server, &tool, handler) },
		IsWriteTool: true,
	}
}
