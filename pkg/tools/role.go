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

type RoleInterface interface {
	List() *Tool
	Get() *Tool
	GetTools() []*Tool
}

type projectRole struct {
	RoleInterface
	client apiClient.ClientInterface
}

func newRole(client apiClient.ClientInterface) RoleInterface {
	return &projectRole{
		client: client,
	}
}

func (r *projectRole) GetTools() []*Tool {
	return []*Tool{
		r.List(),
		r.Get(),
	}
}

type ProjectRoleInput struct {
	Project string `json:"project" jsonschema:"Project name"`
}

func (r *projectRole) List() *Tool {
	tool := mcp.Tool{
		Name:        "perses_list_project_roles",
		Description: "List Roles for a specific project",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Lists roles for a specific project in Perses",
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
				},
			},
			Required: []string{"project"},
		},
	}

	handler := func(ctx context.Context, _ *mcp.CallToolRequest, input ProjectRoleInput) (*mcp.CallToolResult, any, error) {
		roles, err := r.client.Role(input.Project).List("")
		if err != nil {
			return nil, nil, fmt.Errorf("error retrieving roles in project '%s': %w", input.Project, err)
		}

		rolesJSON, err := json.Marshal(roles)
		if err != nil {
			return nil, nil, fmt.Errorf("error marshalling roles: %w", err)
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: string(rolesJSON),
				},
			},
		}, nil, nil
	}

	return &Tool{
		MCPTool:      &tool,
		IsWriteTool:  false,
		ResourceType: "role",
		RegisterWith: func(server *mcp.Server) { mcp.AddTool(server, &tool, handler) },
	}
}

type GetProjectRoleByNameInput struct {
	Project string `json:"project" jsonschema:"Project name"`
	Name    string `json:"name" jsonschema:"Role name"`
}

func (r *projectRole) Get() *Tool {
	tool := mcp.Tool{
		Name:        "perses_get_project_role_by_name",
		Description: "Get a role by name in a specific project",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Gets a role by name in a specific project in Perses",
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
					Description: "Role name",
					MinLength:   jsonschema.Ptr(1),
					MaxLength:   jsonschema.Ptr(75),
					Pattern:     "^[a-zA-Z0-9_.-]+$",
				},
			},
			Required: []string{"project", "name"},
		},
	}

	handler := func(ctx context.Context, _ *mcp.CallToolRequest, input GetProjectRoleByNameInput) (*mcp.CallToolResult, any, error) {
		role, err := r.client.Role(input.Project).Get(input.Name)
		if err != nil {
			return nil, nil, fmt.Errorf("error retrieving role '%s' in project '%s': %w", input.Name, input.Project, err)
		}

		roleJSON, err := json.Marshal(role)
		if err != nil {
			return nil, nil, fmt.Errorf("error marshalling role: %w", err)
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: string(roleJSON),
				},
			},
		}, nil, nil
	}

	return &Tool{
		MCPTool:      &tool,
		IsWriteTool:  false,
		ResourceType: "role",
		RegisterWith: func(server *mcp.Server) { mcp.AddTool(server, &tool, handler) },
	}
}
