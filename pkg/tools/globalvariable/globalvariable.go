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

package globalvariable

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

type globalVariable struct {
	client apiClient.ClientInterface
}

func New(client apiClient.ClientInterface) resource.Resource {
	return &globalVariable{
		client: client,
	}
}

func (g *globalVariable) GetTools() []*tools.Tool {
	return []*tools.Tool{
		g.List(),
		g.Get(),
		g.Create(),
		g.Update(),
		g.Delete(),
	}
}

func (g *globalVariable) List() *tools.Tool {
	tool := &mcp.Tool{
		Name:        "perses_list_global_variables",
		Description: "List all Global Variables",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Lists all global variables in Perses",
			ReadOnlyHint:    true,
			DestructiveHint: jsonschema.Ptr(false),
			IdempotentHint:  true,
			OpenWorldHint:   jsonschema.Ptr(false),
		},
	}

	handler := func(_ context.Context, _ *mcp.CallToolRequest, input map[string]any) (*mcp.CallToolResult, any, error) { //nolint:unparam
		variables, err := g.client.GlobalVariable().List("")
		if err != nil {
			return nil, nil, fmt.Errorf("error retrieving global variables: %w", err)
		}

		variablesJSON, err := json.Marshal(variables)
		if err != nil {
			return nil, nil, fmt.Errorf("error marshalling global variables: %w", err)
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
		ResourceType: tools.GlobalVariableResource,
		RegisterWith: func(server *mcp.Server) { mcp.AddTool(server, tool, handler) },
	}
}

type GetGlobalVariableByNameInput struct {
	Name string `json:"name" jsonschema:"Global Variable name"`
}

func (g *globalVariable) Get() *tools.Tool {
	tool := &mcp.Tool{
		Name:        "perses_get_global_variable_by_name",
		Description: "Get a global variable by name",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Gets a global variable by name in Perses",
			ReadOnlyHint:    true,
			DestructiveHint: jsonschema.Ptr(false),
			IdempotentHint:  true,
			OpenWorldHint:   jsonschema.Ptr(false),
		},
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"name": {
					Type:        "string",
					Description: "Global Variable name",
					MinLength:   jsonschema.Ptr(1),
					MaxLength:   jsonschema.Ptr(75),
					Pattern:     "^[a-zA-Z0-9_.-]+$",
				},
			},
			Required: []string{"name"},
		},
	}

	handler := func(_ context.Context, _ *mcp.CallToolRequest, input GetGlobalVariableByNameInput) (*mcp.CallToolResult, any, error) { //nolint:unparam
		globalVariable, err := g.client.GlobalVariable().Get(input.Name)
		if err != nil {
			return nil, nil, fmt.Errorf("error retrieving global variable '%s': %w", input.Name, err)
		}

		globalVariableJSON, err := json.Marshal(globalVariable)
		if err != nil {
			return nil, nil, fmt.Errorf("error marshalling global variable '%s': %w", input.Name, err)
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: string(globalVariableJSON),
				},
			},
		}, nil, nil
	}

	return &tools.Tool{
		MCPTool:      tool,
		IsWriteTool:  false,
		ResourceType: tools.GlobalVariableResource,
		RegisterWith: func(server *mcp.Server) { mcp.AddTool(server, tool, handler) },
	}
}

type CreateGlobalVariableInput struct {
	Name  string `json:"name" jsonschema:"Global Variable name"`
	Value string `json:"value" jsonschema:"Variable value (for TextVariable)"`
}

func (g *globalVariable) Create() *tools.Tool {
	tool := &mcp.Tool{
		Name:        "perses_create_global_variable",
		Description: "Create a global variable (TextVariable type)",
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"name": {
					Type:        "string",
					Description: "Global Variable name",
					MinLength:   jsonschema.Ptr(1),
					MaxLength:   jsonschema.Ptr(75),
					Pattern:     "^[a-zA-Z0-9_.-]+$",
				},
				"value": {
					Type:        "string",
					Description: "Variable value",
				},
			},
			Required: []string{"name", "value"},
		},
		Annotations: &mcp.ToolAnnotations{
			DestructiveHint: jsonschema.Ptr(false),
			IdempotentHint:  true,
			OpenWorldHint:   jsonschema.Ptr(false),
			ReadOnlyHint:    false,
			Title:           "Creates a global variable in Perses",
		},
	}

	handler := func(_ context.Context, _ *mcp.CallToolRequest, input CreateGlobalVariableInput) (*mcp.CallToolResult, any, error) { //nolint:unparam
		globalVar := &v1.GlobalVariable{
			Kind: v1.KindGlobalVariable,
			Metadata: v1.Metadata{
				Name: input.Name,
			},
			Spec: v1.VariableSpec{
				Kind: variable.KindText,
				Spec: &variable.TextSpec{
					Value: input.Value,
				},
			},
		}

		result, err := g.client.GlobalVariable().Create(globalVar)
		if err != nil {
			return nil, nil, fmt.Errorf("error creating global variable '%s': %w", input.Name, err)
		}

		resultJSON, err := json.Marshal(result)
		if err != nil {
			return nil, nil, fmt.Errorf("error marshalling created global variable: %w", err)
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
		ResourceType: tools.GlobalVariableResource,
		RegisterWith: func(server *mcp.Server) { mcp.AddTool(server, tool, handler) },
	}
}

type UpdateGlobalVariableInput struct {
	Name  string `json:"name" jsonschema:"Global Variable name"`
	Value string `json:"value" jsonschema:"Variable value (for TextVariable)"`
}

func (g *globalVariable) Update() *tools.Tool {
	tool := &mcp.Tool{
		Name:        "perses_update_global_variable",
		Description: "Update an existing global variable (TextVariable type)",
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"name": {
					Type:        "string",
					Description: "Global Variable name",
					MinLength:   jsonschema.Ptr(1),
					MaxLength:   jsonschema.Ptr(75),
					Pattern:     "^[a-zA-Z0-9_.-]+$",
				},
				"value": {
					Type:        "string",
					Description: "Variable value",
				},
			},
			Required: []string{"name", "value"},
		},
		Annotations: &mcp.ToolAnnotations{
			DestructiveHint: jsonschema.Ptr(false),
			IdempotentHint:  true,
			OpenWorldHint:   jsonschema.Ptr(false),
			ReadOnlyHint:    false,
			Title:           "Updates an existing global variable in Perses",
		},
	}

	handler := func(_ context.Context, _ *mcp.CallToolRequest, input UpdateGlobalVariableInput) (*mcp.CallToolResult, any, error) { //nolint:unparam
		globalVar := &v1.GlobalVariable{
			Kind: v1.KindGlobalVariable,
			Metadata: v1.Metadata{
				Name: input.Name,
			},
			Spec: v1.VariableSpec{
				Kind: variable.KindText,
				Spec: &variable.TextSpec{
					Value: input.Value,
				},
			},
		}

		result, err := g.client.GlobalVariable().Update(globalVar)
		if err != nil {
			return nil, nil, fmt.Errorf("error updating global variable '%s': %w", input.Name, err)
		}

		resultJSON, err := json.Marshal(result)
		if err != nil {
			return nil, nil, fmt.Errorf("error marshalling updated global variable: %w", err)
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
		ResourceType: tools.GlobalVariableResource,
		RegisterWith: func(server *mcp.Server) { mcp.AddTool(server, tool, handler) },
	}
}

type DeleteGlobalVariableInput struct {
	Name string `json:"name" jsonschema:"Global Variable name to delete"`
}

func (g *globalVariable) Delete() *tools.Tool {
	tool := &mcp.Tool{
		Name:        "perses_delete_global_variable",
		Description: "Delete a global variable",
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"name": {
					Type:        "string",
					Description: "Global Variable name",
					MinLength:   jsonschema.Ptr(1),
					MaxLength:   jsonschema.Ptr(75),
					Pattern:     "^[a-zA-Z0-9_.-]+$",
				},
			},
			Required: []string{"name"},
		},
		Annotations: &mcp.ToolAnnotations{
			DestructiveHint: jsonschema.Ptr(true),
			IdempotentHint:  true,
			OpenWorldHint:   jsonschema.Ptr(false),
			ReadOnlyHint:    false,
			Title:           "Deletes a global variable in Perses",
		},
	}

	handler := func(_ context.Context, _ *mcp.CallToolRequest, input DeleteGlobalVariableInput) (*mcp.CallToolResult, any, error) { //nolint:unparam
		err := g.client.GlobalVariable().Delete(input.Name)
		if err != nil {
			return nil, nil, fmt.Errorf("error deleting global variable '%s': %w", input.Name, err)
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: fmt.Sprintf("Global variable '%s' deleted successfully", input.Name),
				},
			},
		}, nil, nil
	}

	return &tools.Tool{
		MCPTool:      tool,
		IsWriteTool:  true,
		ResourceType: tools.GlobalVariableResource,
		RegisterWith: func(server *mcp.Server) { mcp.AddTool(server, tool, handler) },
	}
}
