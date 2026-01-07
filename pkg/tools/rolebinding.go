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

type RoleBindingInterface interface {
	List() *Tool
	Get() *Tool
	GetTools() []*Tool
}

type projectRoleBinding struct {
	RoleBindingInterface
	client apiClient.ClientInterface
}

func newRoleBinding(client apiClient.ClientInterface) RoleBindingInterface {
	return &projectRoleBinding{
		client: client,
	}
}

func (r *projectRoleBinding) GetTools() []*Tool {
	return []*Tool{
		r.List(),
		r.Get(),
	}
}

type ProjectRoleBindingInput struct {
	Project string `json:"project" jsonschema:"Project name"`
}

func (r *projectRoleBinding) List() *Tool {
	tool := mcp.Tool{
		Name:        "perses_list_project_role_bindings",
		Description: "List Role Bindings for a specific project",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Lists role bindings for a specific project in Perses",
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

	handler := func(ctx context.Context, _ *mcp.CallToolRequest, input ProjectRoleBindingInput) (*mcp.CallToolResult, any, error) {
		roleBindings, err := r.client.RoleBinding(input.Project).List("")
		if err != nil {
			return nil, nil, fmt.Errorf("error retrieving role bindings in project '%s': %w", input.Project, err)
		}

		roleBindingsJSON, err := json.Marshal(roleBindings)
		if err != nil {
			return nil, nil, fmt.Errorf("error marshalling role bindings: %w", err)
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: string(roleBindingsJSON),
				},
			},
		}, nil, nil
	}

	return &Tool{
		MCPTool:      &tool,
		IsWriteTool:  false,
		ResourceType: "rolebinding",
		RegisterWith: func(server *mcp.Server) { mcp.AddTool(server, &tool, handler) },
	}
}

type GetProjectRoleBindingByNameInput struct {
	Project string `json:"project" jsonschema:"Project name"`
	Name    string `json:"name" jsonschema:"Role Binding name"`
}

func (r *projectRoleBinding) Get() *Tool {
	tool := mcp.Tool{
		Name:        "perses_get_project_role_binding_by_name",
		Description: "Get a role binding by name in a specific project",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Gets a role binding by name in a specific project in Perses",
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
					Description: "Role Binding name",
					MinLength:   jsonschema.Ptr(1),
					MaxLength:   jsonschema.Ptr(75),
					Pattern:     "^[a-zA-Z0-9_.-]+$",
				},
			},
			Required: []string{"project", "name"},
		},
	}

	handler := func(ctx context.Context, _ *mcp.CallToolRequest, input GetProjectRoleBindingByNameInput) (*mcp.CallToolResult, any, error) {
		roleBinding, err := r.client.RoleBinding(input.Project).Get(input.Name)
		if err != nil {
			return nil, nil, fmt.Errorf("error retrieving role binding '%s' in project '%s': %w", input.Name, input.Project, err)
		}

		roleBindingJSON, err := json.Marshal(roleBinding)
		if err != nil {
			return nil, nil, fmt.Errorf("error marshalling role binding: %w", err)
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: string(roleBindingJSON),
				},
			},
		}, nil, nil
	}

	return &Tool{
		MCPTool:      &tool,
		IsWriteTool:  false,
		ResourceType: "rolebinding",
		RegisterWith: func(server *mcp.Server) { mcp.AddTool(server, &tool, handler) },
	}
}
