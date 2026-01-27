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

package globalrolebinding

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

type globalRoleBinding struct {
	client apiClient.ClientInterface
}

func NewGlobalRoleBinding(client apiClient.ClientInterface) resource.Resource {
	return &globalRoleBinding{
		client: client,
	}
}

func (g *globalRoleBinding) GetTools() []*tools.Tool {
	return []*tools.Tool{
		g.List(),
		g.Get(),
		g.Create(),
		g.Update(),
		g.Delete(),
	}
}

func (g *globalRoleBinding) List() *tools.Tool {
	tool := &mcp.Tool{
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

	return &tools.Tool{
		MCPTool:      tool,
		IsWriteTool:  false,
		ResourceType: tools.GlobalRoleBindingResource,
		RegisterWith: func(server *mcp.Server) { mcp.AddTool(server, tool, handler) },
	}
}

type GetGlobalRoleBindingByNameInput struct {
	Name string `json:"name" jsonschema:"Global Role Binding name"`
}

func (g *globalRoleBinding) Get() *tools.Tool {
	tool := &mcp.Tool{
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

	return &tools.Tool{
		MCPTool:      tool,
		IsWriteTool:  false,
		ResourceType: tools.GlobalRoleBindingResource,
		RegisterWith: func(server *mcp.Server) { mcp.AddTool(server, tool, handler) },
	}
}

type CreateGlobalRoleBindingInput struct {
	Name     string   `json:"name" jsonschema:"Global Role Binding name"`
	Role     string   `json:"role" jsonschema:"Name of the GlobalRole to bind"`
	Subjects []string `json:"subjects" jsonschema:"List of user names to bind to the role"`
}

func (g *globalRoleBinding) Create() *tools.Tool {
	tool := &mcp.Tool{
		Name:        "perses_create_global_role_binding",
		Description: "Create a global role binding that binds users to a global role",
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
				"role": {
					Type:        "string",
					Description: "Name of the GlobalRole to bind",
					MinLength:   jsonschema.Ptr(1),
				},
				"subjects": {
					Type:        "array",
					Description: "List of user names to bind to the role",
					Items: &jsonschema.Schema{
						Type: "string",
					},
				},
			},
			Required: []string{"name", "role", "subjects"},
		},
		Annotations: &mcp.ToolAnnotations{
			DestructiveHint: jsonschema.Ptr(false),
			IdempotentHint:  true,
			OpenWorldHint:   jsonschema.Ptr(false),
			ReadOnlyHint:    false,
			Title:           "Creates a global role binding in Perses",
		},
	}

	handler := func(ctx context.Context, _ *mcp.CallToolRequest, input CreateGlobalRoleBindingInput) (*mcp.CallToolResult, any, error) {
		subjects := make([]v1.Subject, len(input.Subjects))
		for i, s := range input.Subjects {
			subjects[i] = v1.Subject{
				Kind: v1.KindUser,
				Name: s,
			}
		}

		globalRoleBindingObj := &v1.GlobalRoleBinding{
			Kind: v1.KindGlobalRoleBinding,
			Metadata: v1.Metadata{
				Name: input.Name,
			},
			Spec: v1.RoleBindingSpec{
				Role:     input.Role,
				Subjects: subjects,
			},
		}

		result, err := g.client.GlobalRoleBinding().Create(globalRoleBindingObj)
		if err != nil {
			return nil, nil, fmt.Errorf("error creating global role binding '%s': %w", input.Name, err)
		}

		resultJSON, err := json.Marshal(result)
		if err != nil {
			return nil, nil, fmt.Errorf("error marshalling created global role binding: %w", err)
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
		ResourceType: tools.GlobalRoleBindingResource,
		RegisterWith: func(server *mcp.Server) { mcp.AddTool(server, tool, handler) },
	}
}

type UpdateGlobalRoleBindingInput struct {
	Name     string   `json:"name" jsonschema:"Global Role Binding name"`
	Role     string   `json:"role" jsonschema:"Name of the GlobalRole to bind"`
	Subjects []string `json:"subjects" jsonschema:"List of user names to bind to the role"`
}

func (g *globalRoleBinding) Update() *tools.Tool {
	tool := &mcp.Tool{
		Name:        "perses_update_global_role_binding",
		Description: "Update an existing global role binding",
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
				"role": {
					Type:        "string",
					Description: "Name of the GlobalRole to bind",
					MinLength:   jsonschema.Ptr(1),
				},
				"subjects": {
					Type:        "array",
					Description: "List of user names to bind to the role",
					Items: &jsonschema.Schema{
						Type: "string",
					},
				},
			},
			Required: []string{"name", "role", "subjects"},
		},
		Annotations: &mcp.ToolAnnotations{
			DestructiveHint: jsonschema.Ptr(false),
			IdempotentHint:  true,
			OpenWorldHint:   jsonschema.Ptr(false),
			ReadOnlyHint:    false,
			Title:           "Updates an existing global role binding in Perses",
		},
	}

	handler := func(ctx context.Context, _ *mcp.CallToolRequest, input UpdateGlobalRoleBindingInput) (*mcp.CallToolResult, any, error) {
		subjects := make([]v1.Subject, len(input.Subjects))
		for i, s := range input.Subjects {
			subjects[i] = v1.Subject{
				Kind: v1.KindUser,
				Name: s,
			}
		}

		globalRoleBindingObj := &v1.GlobalRoleBinding{
			Kind: v1.KindGlobalRoleBinding,
			Metadata: v1.Metadata{
				Name: input.Name,
			},
			Spec: v1.RoleBindingSpec{
				Role:     input.Role,
				Subjects: subjects,
			},
		}

		result, err := g.client.GlobalRoleBinding().Update(globalRoleBindingObj)
		if err != nil {
			return nil, nil, fmt.Errorf("error updating global role binding '%s': %w", input.Name, err)
		}

		resultJSON, err := json.Marshal(result)
		if err != nil {
			return nil, nil, fmt.Errorf("error marshalling updated global role binding: %w", err)
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
		ResourceType: tools.GlobalRoleBindingResource,
		RegisterWith: func(server *mcp.Server) { mcp.AddTool(server, tool, handler) },
	}
}

type DeleteGlobalRoleBindingInput struct {
	Name string `json:"name" jsonschema:"Global Role Binding name to delete"`
}

func (g *globalRoleBinding) Delete() *tools.Tool {
	tool := &mcp.Tool{
		Name:        "perses_delete_global_role_binding",
		Description: "Delete a global role binding",
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
		Annotations: &mcp.ToolAnnotations{
			DestructiveHint: jsonschema.Ptr(true),
			IdempotentHint:  true,
			OpenWorldHint:   jsonschema.Ptr(false),
			ReadOnlyHint:    false,
			Title:           "Deletes a global role binding in Perses",
		},
	}

	handler := func(ctx context.Context, _ *mcp.CallToolRequest, input DeleteGlobalRoleBindingInput) (*mcp.CallToolResult, any, error) {
		err := g.client.GlobalRoleBinding().Delete(input.Name)
		if err != nil {
			return nil, nil, fmt.Errorf("error deleting global role binding '%s': %w", input.Name, err)
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: fmt.Sprintf("Global role binding '%s' deleted successfully", input.Name),
				},
			},
		}, nil, nil
	}

	return &tools.Tool{
		MCPTool:      tool,
		IsWriteTool:  true,
		ResourceType: tools.GlobalRoleBindingResource,
		RegisterWith: func(server *mcp.Server) { mcp.AddTool(server, tool, handler) },
	}
}
