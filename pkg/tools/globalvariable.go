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
)

type GlobalVariableInterface interface {
	List() *Tool
	Get() *Tool
	GetTools() []*Tool
}

type globalVariable struct {
	GlobalVariableInterface
	client apiClient.ClientInterface
}

func newGlobalVariable(client apiClient.ClientInterface) GlobalVariableInterface {
	return &globalVariable{
		client: client,
	}
}

func (g *globalVariable) GetTools() []*Tool {
	return []*Tool{
		g.List(),
		g.Get(),
	}
}

func (g *globalVariable) List() *Tool {
	tool := mcp.Tool{
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

	handler := func(ctx context.Context, _ *mcp.CallToolRequest, input map[string]any) (*mcp.CallToolResult, any, error) {
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

	return &Tool{
		MCPTool:      &tool,
		IsWriteTool:  false,
		ResourceType: "globalvariable",
		RegisterWith: func(server *mcp.Server) { mcp.AddTool(server, &tool, handler) },
	}
}

type GetGlobalVariableByNameInput struct {
	Name string `json:"name" jsonschema:"Global Variable name"`
}

func (g *globalVariable) Get() *Tool {
	tool := mcp.Tool{
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

	handler := func(ctx context.Context, _ *mcp.CallToolRequest, input GetGlobalVariableByNameInput) (*mcp.CallToolResult, any, error) {
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

	return &Tool{
		MCPTool:      &tool,
		IsWriteTool:  false,
		ResourceType: "globalvariable",
		RegisterWith: func(server *mcp.Server) { mcp.AddTool(server, &tool, handler) },
	}
}
