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
)

type globalRole struct {
	client apiClient.ClientInterface
}

func NewGlobalRole(client apiClient.ClientInterface) resource.Resource {
	return &globalRole{
		client: client,
	}
}

func (g *globalRole) GetTools() []*tools.Tool {
	return []*tools.Tool{
		g.List(),
		g.Get(),
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

	handler := func(ctx context.Context, _ *mcp.CallToolRequest, input map[string]any) (*mcp.CallToolResult, any, error) {
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

	handler := func(ctx context.Context, _ *mcp.CallToolRequest, input GetGlobalRoleByNameInput) (*mcp.CallToolResult, any, error) {
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

// Create is not yet implemented for global role
func (g *globalRole) Create() *tools.Tool {
	return nil
}

// Update is not yet implemented for global role
func (g *globalRole) Update() *tools.Tool {
	return nil
}

// Delete is not yet implemented for global role
func (g *globalRole) Delete() *tools.Tool {
	return nil
}
