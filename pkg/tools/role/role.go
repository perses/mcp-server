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

package role

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
	roleModel "github.com/perses/perses/pkg/model/api/v1/role"
)

type role struct {
	client apiClient.ClientInterface
}

func New(client apiClient.ClientInterface) resource.Resource {
	return &role{
		client: client,
	}
}

func (r *role) GetTools() []*tools.Tool {
	return []*tools.Tool{
		r.List(),
		r.Get(),
		r.Create(),
		r.Update(),
		r.Delete(),
	}
}

type ProjectRoleInput struct {
	Project string `json:"project" jsonschema:"Project name"`
}

func (r *role) List() *tools.Tool {
	tool := &mcp.Tool{
		Name:        "perses_list_project_roles",
		Description: "List the roles of one project with their permissions (actions + scopes). Roles grant nothing until bound to users with a role binding (perses_list_project_role_bindings). Only available when authorization is enabled on the Perses server (config security.enable_auth=true); otherwise the API answers 404 'document not found' (reads) or 405 'Method Not Allowed' (writes), which does not mean the project or object is missing.",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Lists roles for a specific project in Perses",
			ReadOnlyHint:    true,
			DestructiveHint: new(false),
			IdempotentHint:  true,
			OpenWorldHint:   new(false),
		},
		InputSchema: &jsonschema.Schema{
			Type: tools.SchemaTypeObject,
			Properties: map[string]*jsonschema.Schema{
				string(tools.ProjectResource): {
					Type:        tools.SchemaTypeString,
					Description: "Project name",
					MinLength:   new(1),
				},
			},
			Required: []string{string(tools.ProjectResource)},
		},
	}

	handler := func(_ context.Context, _ *mcp.CallToolRequest, input ProjectRoleInput) (*mcp.CallToolResult, any, error) { //nolint:unparam
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

	return &tools.Tool{
		MCPTool:      tool,
		IsWriteTool:  false,
		ResourceType: tools.RoleResource,
		RegisterWith: func(server *mcp.Server) { mcp.AddTool(server, tool, handler) },
	}
}

type GetProjectRoleByNameInput struct {
	Project string `json:"project" jsonschema:"Project name"`
	Name    string `json:"name" jsonschema:"Role name"`
}

func (r *role) Get() *tools.Tool {
	tool := &mcp.Tool{
		Name:        "perses_get_project_role_by_name",
		Description: "Get one project role by name with its permissions (actions + scopes).",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Gets a role by name in a specific project in Perses",
			ReadOnlyHint:    true,
			DestructiveHint: new(false),
			IdempotentHint:  true,
			OpenWorldHint:   new(false),
		},
		InputSchema: &jsonschema.Schema{
			Type: tools.SchemaTypeObject,
			Properties: map[string]*jsonschema.Schema{
				string(tools.ProjectResource): {
					Type:        tools.SchemaTypeString,
					Description: "Project name",
					MinLength:   new(1),
					MaxLength:   new(75),
					Pattern:     tools.PatternResourceName,
				},
				tools.ResourceName: {
					Type:        tools.SchemaTypeString,
					Description: "Role name",
					MinLength:   new(1),
					MaxLength:   new(75),
					Pattern:     tools.PatternResourceName,
				},
			},
			Required: []string{string(tools.ProjectResource), tools.ResourceName},
		},
	}

	handler := func(_ context.Context, _ *mcp.CallToolRequest, input GetProjectRoleByNameInput) (*mcp.CallToolResult, any, error) { //nolint:unparam
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

	return &tools.Tool{
		MCPTool:      tool,
		IsWriteTool:  false,
		ResourceType: tools.RoleResource,
		RegisterWith: func(server *mcp.Server) { mcp.AddTool(server, tool, handler) },
	}
}

type CreateProjectRoleInput struct {
	Project string   `json:"project" jsonschema:"Project name"`
	Name    string   `json:"name" jsonschema:"Role name"`
	Actions []string `json:"actions" jsonschema:"Actions granted: read, create, update, delete, or * for all"`
	Scopes  []string `json:"scopes" jsonschema:"Project resource kinds the actions apply to: Dashboard, Datasource, EphemeralDashboard, Folder, Project, Role, RoleBinding, Secret, Variable, or * for all"`
}

func (r *role) Create() *tools.Tool {
	tool := &mcp.Tool{
		Name:        "perses_create_project_role",
		Description: "Create a project role with a single permission made of actions (read, create, update, delete or *) applied to project-level scopes (Dashboard, Datasource, EphemeralDashboard, Folder, Project, Role, RoleBinding, Secret, Variable or *). For read-only access to dashboards use actions=[\"read\"], scopes=[\"Dashboard\"]. Bind it to users with perses_create_project_role_binding. Fails if the role exists. Only available when authorization is enabled on the Perses server (config security.enable_auth=true); otherwise the API answers 404 'document not found' (reads) or 405 'Method Not Allowed' (writes), which does not mean the project or object is missing.",
		InputSchema: &jsonschema.Schema{
			Type: tools.SchemaTypeObject,
			Properties: map[string]*jsonschema.Schema{
				string(tools.ProjectResource): {
					Type:        tools.SchemaTypeString,
					Description: "Project name",
					MinLength:   new(1),
					MaxLength:   new(75),
					Pattern:     tools.PatternResourceName,
				},
				tools.ResourceName: {
					Type:        tools.SchemaTypeString,
					Description: "Role name",
					MinLength:   new(1),
					MaxLength:   new(75),
					Pattern:     tools.PatternResourceName,
				},
				"actions": {
					Type:        tools.SchemaTypeArray,
					Description: "Actions granted: read, create, update, delete, or * for all",
					Items: &jsonschema.Schema{
						Type: tools.SchemaTypeString,
					},
				},
				"scopes": {
					Type:        tools.SchemaTypeArray,
					Description: "Project resource kinds the actions apply to: Dashboard, Datasource, EphemeralDashboard, Folder, Project, Role, RoleBinding, Secret, Variable, or * for all",
					Items: &jsonschema.Schema{
						Type: tools.SchemaTypeString,
					},
				},
			},
			Required: []string{string(tools.ProjectResource), tools.ResourceName, "actions", "scopes"},
		},
		Annotations: &mcp.ToolAnnotations{
			DestructiveHint: new(false),
			IdempotentHint:  true,
			OpenWorldHint:   new(false),
			ReadOnlyHint:    false,
			Title:           "Creates a project role in Perses",
		},
	}

	handler := func(_ context.Context, _ *mcp.CallToolRequest, input CreateProjectRoleInput) (*mcp.CallToolResult, any, error) { //nolint:unparam
		actions := make([]roleModel.Action, len(input.Actions))
		for i, a := range input.Actions {
			actions[i] = roleModel.Action(a)
		}
		scopes := make([]roleModel.Scope, len(input.Scopes))
		for i, s := range input.Scopes {
			scopes[i] = roleModel.Scope(s)
		}

		roleObj := &v1.Role{
			Kind: v1.KindRole,
			Metadata: v1.ProjectMetadata{
				Metadata: v1.Metadata{
					Name: input.Name,
				},
				ProjectMetadataWrapper: v1.ProjectMetadataWrapper{
					Project: input.Project,
				},
			},
			Spec: v1.RoleSpec{
				Permissions: []roleModel.Permission{
					{
						Actions: actions,
						Scopes:  scopes,
					},
				},
			},
		}

		result, err := r.client.Role(input.Project).Create(roleObj)
		if err != nil {
			return nil, nil, fmt.Errorf("error creating role '%s' in project '%s': %w", input.Name, input.Project, err)
		}

		resultJSON, err := json.Marshal(result)
		if err != nil {
			return nil, nil, fmt.Errorf("error marshalling created role: %w", err)
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
		ResourceType: tools.RoleResource,
		RegisterWith: func(server *mcp.Server) { mcp.AddTool(server, tool, handler) },
	}
}

type UpdateProjectRoleInput struct {
	Project string   `json:"project" jsonschema:"Project name"`
	Name    string   `json:"name" jsonschema:"Role name"`
	Actions []string `json:"actions" jsonschema:"Actions granted: read, create, update, delete, or * for all"`
	Scopes  []string `json:"scopes" jsonschema:"Project resource kinds the actions apply to: Dashboard, Datasource, EphemeralDashboard, Folder, Project, Role, RoleBinding, Secret, Variable, or * for all"`
}

func (r *role) Update() *tools.Tool {
	tool := &mcp.Tool{
		Name:        "perses_update_project_role",
		Description: "Replace the permissions of an existing project role with the given actions and scopes (same values as perses_create_project_role). Previous permissions are discarded, so include everything the role should keep.",
		InputSchema: &jsonschema.Schema{
			Type: tools.SchemaTypeObject,
			Properties: map[string]*jsonschema.Schema{
				string(tools.ProjectResource): {
					Type:        tools.SchemaTypeString,
					Description: "Project name",
					MinLength:   new(1),
					MaxLength:   new(75),
					Pattern:     tools.PatternResourceName,
				},
				tools.ResourceName: {
					Type:        tools.SchemaTypeString,
					Description: "Role name",
					MinLength:   new(1),
					MaxLength:   new(75),
					Pattern:     tools.PatternResourceName,
				},
				"actions": {
					Type:        tools.SchemaTypeArray,
					Description: "Actions granted: read, create, update, delete, or * for all",
					Items: &jsonschema.Schema{
						Type: tools.SchemaTypeString,
					},
				},
				"scopes": {
					Type:        tools.SchemaTypeArray,
					Description: "Project resource kinds the actions apply to: Dashboard, Datasource, EphemeralDashboard, Folder, Project, Role, RoleBinding, Secret, Variable, or * for all",
					Items: &jsonschema.Schema{
						Type: tools.SchemaTypeString,
					},
				},
			},
			Required: []string{string(tools.ProjectResource), tools.ResourceName, "actions", "scopes"},
		},
		Annotations: &mcp.ToolAnnotations{
			DestructiveHint: new(false),
			IdempotentHint:  true,
			OpenWorldHint:   new(false),
			ReadOnlyHint:    false,
			Title:           "Updates an existing project role in Perses",
		},
	}

	handler := func(_ context.Context, _ *mcp.CallToolRequest, input UpdateProjectRoleInput) (*mcp.CallToolResult, any, error) { //nolint:unparam
		actions := make([]roleModel.Action, len(input.Actions))
		for i, a := range input.Actions {
			actions[i] = roleModel.Action(a)
		}
		scopes := make([]roleModel.Scope, len(input.Scopes))
		for i, s := range input.Scopes {
			scopes[i] = roleModel.Scope(s)
		}

		roleObj := &v1.Role{
			Kind: v1.KindRole,
			Metadata: v1.ProjectMetadata{
				Metadata: v1.Metadata{
					Name: input.Name,
				},
				ProjectMetadataWrapper: v1.ProjectMetadataWrapper{
					Project: input.Project,
				},
			},
			Spec: v1.RoleSpec{
				Permissions: []roleModel.Permission{
					{
						Actions: actions,
						Scopes:  scopes,
					},
				},
			},
		}

		result, err := r.client.Role(input.Project).Update(roleObj)
		if err != nil {
			return nil, nil, fmt.Errorf("error updating role '%s' in project '%s': %w", input.Name, input.Project, err)
		}

		resultJSON, err := json.Marshal(result)
		if err != nil {
			return nil, nil, fmt.Errorf("error marshalling updated role: %w", err)
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
		ResourceType: tools.RoleResource,
		RegisterWith: func(server *mcp.Server) { mcp.AddTool(server, tool, handler) },
	}
}

type DeleteProjectRoleInput struct {
	Project string `json:"project" jsonschema:"Project name"`
	Name    string `json:"name" jsonschema:"Role name to delete"`
}

func (r *role) Delete() *tools.Tool {
	tool := &mcp.Tool{
		Name:        "perses_delete_project_role",
		Description: "Permanently delete a project role. Role bindings referencing it stop granting access.",
		InputSchema: &jsonschema.Schema{
			Type: tools.SchemaTypeObject,
			Properties: map[string]*jsonschema.Schema{
				string(tools.ProjectResource): {
					Type:        tools.SchemaTypeString,
					Description: "Project name",
					MinLength:   new(1),
					MaxLength:   new(75),
					Pattern:     tools.PatternResourceName,
				},
				tools.ResourceName: {
					Type:        tools.SchemaTypeString,
					Description: "Role name",
					MinLength:   new(1),
					MaxLength:   new(75),
					Pattern:     tools.PatternResourceName,
				},
			},
			Required: []string{string(tools.ProjectResource), tools.ResourceName},
		},
		Annotations: &mcp.ToolAnnotations{
			DestructiveHint: new(true),
			IdempotentHint:  true,
			OpenWorldHint:   new(false),
			ReadOnlyHint:    false,
			Title:           "Deletes a project role in Perses",
		},
	}

	handler := func(_ context.Context, _ *mcp.CallToolRequest, input DeleteProjectRoleInput) (*mcp.CallToolResult, any, error) { //nolint:unparam
		err := r.client.Role(input.Project).Delete(input.Name)
		if err != nil {
			return nil, nil, fmt.Errorf("error deleting role '%s' in project '%s': %w", input.Name, input.Project, err)
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: fmt.Sprintf("Role '%s' deleted successfully from project '%s'", input.Name, input.Project),
				},
			},
		}, nil, nil
	}

	return &tools.Tool{
		MCPTool:      tool,
		IsWriteTool:  true,
		ResourceType: tools.RoleResource,
		RegisterWith: func(server *mcp.Server) { mcp.AddTool(server, tool, handler) },
	}
}
