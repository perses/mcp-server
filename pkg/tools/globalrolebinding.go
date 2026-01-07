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

type GlobalRoleBindingInterface interface {
	List() *Tool
	Get() *Tool
	GetTools() []*Tool
}

type globalRoleBinding struct {
	GlobalRoleBindingInterface
	client apiClient.ClientInterface
}

func newGlobalRoleBinding(client apiClient.ClientInterface) GlobalRoleBindingInterface {
	return &globalRoleBinding{
		client: client,
	}
}

func (g *globalRoleBinding) GetTools() []*Tool {
	return []*Tool{
		g.List(),
		g.Get(),
	}
}

func (g *globalRoleBinding) List() *Tool {
	tool := mcp.Tool{
		Name:        "perses_list_global_role_bindings",
		Description: "List all Perses Global Role Bindings",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Lists all global role bindings in Perses",
			ReadOnlyHint:    true,
			DestructiveHint: jsonschema.Ptr(false),
			IdempotentHint:  true,
			OpenWorldHint:   jsonschema.Ptr(false),
		},
	}

	handler := func(ctx context.Context, _ *mcp.CallToolRequest, input map[string]any) (*mcp.CallToolResult, any, error) {
		globalRoleBindings, err := g.client.GlobalRoleBinding().List("")
		if err != nil {
			return nil, nil, fmt.Errorf("error retrieving global role bindings: %w", err)
		}

		globalRoleBindingsJSON, err := json.Marshal(globalRoleBindings)
		if err != nil {
			return nil, nil, fmt.Errorf("error marshalling global role bindings: %w", err)
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: string(globalRoleBindingsJSON),
				},
			},
		}, nil, nil
	}

	return &Tool{
		MCPTool:      &tool,
		IsWriteTool:  false,
		ResourceType: "globalrolebinding",
		RegisterWith: func(server *mcp.Server) { mcp.AddTool(server, &tool, handler) },
	}
}

type GetGlobalRoleBindingByNameInput struct {
	Name string `json:"name" jsonschema:"Global Role Binding name"`
}

func (g *globalRoleBinding) Get() *Tool {
	tool := mcp.Tool{
		Name:        "perses_get_global_role_binding_by_name",
		Description: "Get a global role binding by name",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Gets a global role binding by name in Perses",
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
					Description: "Global Role Binding name",
					MinLength:   jsonschema.Ptr(1),
					MaxLength:   jsonschema.Ptr(75),
					Pattern:     "^[a-zA-Z0-9_.-]+$",
				},
			},
			Required: []string{"name"},
		},
	}

	handler := func(ctx context.Context, _ *mcp.CallToolRequest, input GetGlobalRoleBindingByNameInput) (*mcp.CallToolResult, any, error) {
		globalRoleBinding, err := g.client.GlobalRoleBinding().Get(input.Name)
		if err != nil {
			return nil, nil, fmt.Errorf("error retrieving global role binding '%s': %w", input.Name, err)
		}

		globalRoleBindingJSON, err := json.Marshal(globalRoleBinding)
		if err != nil {
			return nil, nil, fmt.Errorf("error marshalling global role binding '%s': %w", input.Name, err)
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: string(globalRoleBindingJSON),
				},
			},
		}, nil, nil
	}

	return &Tool{
		MCPTool:      &tool,
		IsWriteTool:  false,
		ResourceType: "globalrolebinding",
		RegisterWith: func(server *mcp.Server) { mcp.AddTool(server, &tool, handler) },
	}
}
