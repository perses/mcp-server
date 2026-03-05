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

package globalrole

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
	"github.com/perses/perses/pkg/model/api/v1/role"
)

type globalRole struct {
	client apiClient.ClientInterface
}

func New(client apiClient.ClientInterface) resource.Resource {
	return &globalRole{
		client: client,
	}
}

func (g *globalRole) GetTools() []*tools.Tool {
	return []*tools.Tool{
		g.List(),
		g.Get(),
		g.Create(),
		g.Update(),
		g.Delete(),
	}
}

func (g *globalRole) List() *tools.Tool {
	tool := &mcp.Tool{
		Name:        "perses_list_global_roles",
		Description: "List all Perses Global Roles",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Lists all global roles in Perses",
			ReadOnlyHint:    true,
			DestructiveHint: jsonschema.Ptr(false),
			IdempotentHint:  true,
			OpenWorldHint:   jsonschema.Ptr(false),
		},
	}

	handler := func(_ context.Context, _ *mcp.CallToolRequest, input map[string]any) (*mcp.CallToolResult, any, error) { //nolint: unparam
		globalRoles, err := g.client.GlobalRole().List("")
		if err != nil {
			return nil, nil, fmt.Errorf("error retrieving global roles: %w", err)
		}

		globalRolesJSON, err := json.Marshal(globalRoles)
		if err != nil {
			return nil, nil, fmt.Errorf("error marshalling global roles: %w", err)
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: string(globalRolesJSON),
				},
			},
		}, nil, nil
	}

	return &tools.Tool{
		MCPTool:      tool,
		IsWriteTool:  false,
		ResourceType: tools.GlobalRoleResource,
		RegisterWith: func(server *mcp.Server) { mcp.AddTool(server, tool, handler) },
	}
}

type GetGlobalRoleByNameInput struct {
	Name string `json:"name" jsonschema:"Global Role name"`
}

func (g *globalRole) Get() *tools.Tool {
	tool := &mcp.Tool{
		Name:        "perses_get_global_role_by_name",
		Description: "Get a global role by name",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Gets a global role by name in Perses",
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
					Description: "Global Role name",
					MinLength:   jsonschema.Ptr(1),
					MaxLength:   jsonschema.Ptr(75),
					Pattern:     "^[a-zA-Z0-9_.-]+$",
				},
			},
			Required: []string{"name"},
		},
	}

	handler := func(_ context.Context, _ *mcp.CallToolRequest, input GetGlobalRoleByNameInput) (*mcp.CallToolResult, any, error) { //nolint: unparam
		globalRole, err := g.client.GlobalRole().Get(input.Name)
		if err != nil {
			return nil, nil, fmt.Errorf("error retrieving global role '%s': %w", input.Name, err)
		}

		globalRoleJSON, err := json.Marshal(globalRole)
		if err != nil {
			return nil, nil, fmt.Errorf("error marshalling global role '%s': %w", input.Name, err)
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: string(globalRoleJSON),
				},
			},
		}, nil, nil
	}

	return &tools.Tool{
		MCPTool:      tool,
		IsWriteTool:  false,
		ResourceType: tools.GlobalRoleResource,
		RegisterWith: func(server *mcp.Server) { mcp.AddTool(server, tool, handler) },
	}
}

type CreateGlobalRoleInput struct {
	Name    string   `json:"name" jsonschema:"Global Role name"`
	Actions []string `json:"actions" jsonschema:"List of actions (e.g., read, create, update, delete)"`
	Scopes  []string `json:"scopes" jsonschema:"List of scopes (resource kinds the role applies to)"`
}

func (g *globalRole) Create() *tools.Tool {
	tool := &mcp.Tool{
		Name:        "perses_create_global_role",
		Description: "Create a global role with specified permissions",
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"name": {
					Type:        "string",
					Description: "Global Role name",
					MinLength:   jsonschema.Ptr(1),
					MaxLength:   jsonschema.Ptr(75),
					Pattern:     "^[a-zA-Z0-9_.-]+$",
				},
				"actions": {
					Type:        "array",
					Description: "List of actions (e.g., read, create, update, delete)",
					Items: &jsonschema.Schema{
						Type: "string",
					},
				},
				"scopes": {
					Type:        "array",
					Description: "List of scopes (resource kinds the role applies to)",
					Items: &jsonschema.Schema{
						Type: "string",
					},
				},
			},
			Required: []string{"name", "actions", "scopes"},
		},
		Annotations: &mcp.ToolAnnotations{
			DestructiveHint: jsonschema.Ptr(false),
			IdempotentHint:  true,
			OpenWorldHint:   jsonschema.Ptr(false),
			ReadOnlyHint:    false,
			Title:           "Creates a global role in Perses",
		},
	}

	handler := func(_ context.Context, _ *mcp.CallToolRequest, input CreateGlobalRoleInput) (*mcp.CallToolResult, any, error) { //nolint: unparam
		actions := make([]role.Action, len(input.Actions))
		for i, a := range input.Actions {
			actions[i] = role.Action(a)
		}
		scopes := make([]role.Scope, len(input.Scopes))
		for i, s := range input.Scopes {
			scopes[i] = role.Scope(s)
		}

		globalRoleObj := &v1.GlobalRole{
			Kind: v1.KindGlobalRole,
			Metadata: v1.Metadata{
				Name: input.Name,
			},
			Spec: v1.RoleSpec{
				Permissions: []role.Permission{
					{
						Actions: actions,
						Scopes:  scopes,
					},
				},
			},
		}

		result, err := g.client.GlobalRole().Create(globalRoleObj)
		if err != nil {
			return nil, nil, fmt.Errorf("error creating global role '%s': %w", input.Name, err)
		}

		resultJSON, err := json.Marshal(result)
		if err != nil {
			return nil, nil, fmt.Errorf("error marshalling created global role: %w", err)
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
		ResourceType: tools.GlobalRoleResource,
		RegisterWith: func(server *mcp.Server) { mcp.AddTool(server, tool, handler) },
	}
}

type UpdateGlobalRoleInput struct {
	Name    string   `json:"name" jsonschema:"Global Role name"`
	Actions []string `json:"actions" jsonschema:"List of actions (e.g., read, create, update, delete)"`
	Scopes  []string `json:"scopes" jsonschema:"List of scopes (resource kinds the role applies to)"`
}

func (g *globalRole) Update() *tools.Tool {
	tool := &mcp.Tool{
		Name:        "perses_update_global_role",
		Description: "Update an existing global role with specified permissions",
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"name": {
					Type:        "string",
					Description: "Global Role name",
					MinLength:   jsonschema.Ptr(1),
					MaxLength:   jsonschema.Ptr(75),
					Pattern:     "^[a-zA-Z0-9_.-]+$",
				},
				"actions": {
					Type:        "array",
					Description: "List of actions (e.g., read, create, update, delete)",
					Items: &jsonschema.Schema{
						Type: "string",
					},
				},
				"scopes": {
					Type:        "array",
					Description: "List of scopes (resource kinds the role applies to)",
					Items: &jsonschema.Schema{
						Type: "string",
					},
				},
			},
			Required: []string{"name", "actions", "scopes"},
		},
		Annotations: &mcp.ToolAnnotations{
			DestructiveHint: jsonschema.Ptr(false),
			IdempotentHint:  true,
			OpenWorldHint:   jsonschema.Ptr(false),
			ReadOnlyHint:    false,
			Title:           "Updates an existing global role in Perses",
		},
	}

	handler := func(_ context.Context, _ *mcp.CallToolRequest, input UpdateGlobalRoleInput) (*mcp.CallToolResult, any, error) { //nolint: unparam
		actions := make([]role.Action, len(input.Actions))
		for i, a := range input.Actions {
			actions[i] = role.Action(a)
		}
		scopes := make([]role.Scope, len(input.Scopes))
		for i, s := range input.Scopes {
			scopes[i] = role.Scope(s)
		}

		globalRoleObj := &v1.GlobalRole{
			Kind: v1.KindGlobalRole,
			Metadata: v1.Metadata{
				Name: input.Name,
			},
			Spec: v1.RoleSpec{
				Permissions: []role.Permission{
					{
						Actions: actions,
						Scopes:  scopes,
					},
				},
			},
		}

		result, err := g.client.GlobalRole().Update(globalRoleObj)
		if err != nil {
			return nil, nil, fmt.Errorf("error updating global role '%s': %w", input.Name, err)
		}

		resultJSON, err := json.Marshal(result)
		if err != nil {
			return nil, nil, fmt.Errorf("error marshalling updated global role: %w", err)
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
		ResourceType: tools.GlobalRoleResource,
		RegisterWith: func(server *mcp.Server) { mcp.AddTool(server, tool, handler) },
	}
}

type DeleteGlobalRoleInput struct {
	Name string `json:"name" jsonschema:"Global Role name to delete"`
}

func (g *globalRole) Delete() *tools.Tool {
	tool := &mcp.Tool{
		Name:        "perses_delete_global_role",
		Description: "Delete a global role",
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"name": {
					Type:        "string",
					Description: "Global Role name",
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
			Title:           "Deletes a global role in Perses",
		},
	}

	handler := func(_ context.Context, _ *mcp.CallToolRequest, input DeleteGlobalRoleInput) (*mcp.CallToolResult, any, error) { //nolint: unparam
		err := g.client.GlobalRole().Delete(input.Name)
		if err != nil {
			return nil, nil, fmt.Errorf("error deleting global role '%s': %w", input.Name, err)
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: fmt.Sprintf("Global role '%s' deleted successfully", input.Name),
				},
			},
		}, nil, nil
	}

	return &tools.Tool{
		MCPTool:      tool,
		IsWriteTool:  true,
		ResourceType: tools.GlobalRoleResource,
		RegisterWith: func(server *mcp.Server) { mcp.AddTool(server, tool, handler) },
	}
}
