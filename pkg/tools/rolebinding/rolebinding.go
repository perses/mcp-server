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

package rolebinding

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

type roleBinding struct {
	client apiClient.ClientInterface
}

func New(client apiClient.ClientInterface) resource.Resource {
	return &roleBinding{
		client: client,
	}
}

func (r *roleBinding) GetTools() []*tools.Tool {
	return []*tools.Tool{
		r.List(),
		r.Get(),
		r.Create(),
		r.Update(),
		r.Delete(),
	}
}

type ProjectRoleBindingInput struct {
	Project string `json:"project" jsonschema:"Project name"`
}

func (r *roleBinding) List() *tools.Tool {
	tool := &mcp.Tool{
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

	return &tools.Tool{
		MCPTool:      tool,
		IsWriteTool:  false,
		ResourceType: tools.RoleBindingResource,
		RegisterWith: func(server *mcp.Server) { mcp.AddTool(server, tool, handler) },
	}
}

type GetProjectRoleBindingByNameInput struct {
	Project string `json:"project" jsonschema:"Project name"`
	Name    string `json:"name" jsonschema:"Role Binding name"`
}

func (r *roleBinding) Get() *tools.Tool {
	tool := &mcp.Tool{
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

	return &tools.Tool{
		MCPTool:      tool,
		IsWriteTool:  false,
		ResourceType: tools.RoleBindingResource,
		RegisterWith: func(server *mcp.Server) { mcp.AddTool(server, tool, handler) },
	}
}

type CreateProjectRoleBindingInput struct {
	Project  string   `json:"project" jsonschema:"Project name"`
	Name     string   `json:"name" jsonschema:"Role Binding name"`
	Role     string   `json:"role" jsonschema:"Name of the Role to bind"`
	Subjects []string `json:"subjects" jsonschema:"List of user names to bind to the role"`
}

func (r *roleBinding) Create() *tools.Tool {
	tool := &mcp.Tool{
		Name:        "perses_create_project_role_binding",
		Description: "Create a project role binding that binds users to a role",
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
				"role": {
					Type:        "string",
					Description: "Name of the Role to bind",
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
			Required: []string{"project", "name", "role", "subjects"},
		},
		Annotations: &mcp.ToolAnnotations{
			DestructiveHint: jsonschema.Ptr(false),
			IdempotentHint:  true,
			OpenWorldHint:   jsonschema.Ptr(false),
			ReadOnlyHint:    false,
			Title:           "Creates a project role binding in Perses",
		},
	}

	handler := func(ctx context.Context, _ *mcp.CallToolRequest, input CreateProjectRoleBindingInput) (*mcp.CallToolResult, any, error) {
		subjects := make([]v1.Subject, len(input.Subjects))
		for i, s := range input.Subjects {
			subjects[i] = v1.Subject{
				Kind: v1.KindUser,
				Name: s,
			}
		}

		roleBindingObj := &v1.RoleBinding{
			Kind: v1.KindRoleBinding,
			Metadata: v1.ProjectMetadata{
				Metadata: v1.Metadata{
					Name: input.Name,
				},
				ProjectMetadataWrapper: v1.ProjectMetadataWrapper{
					Project: input.Project,
				},
			},
			Spec: v1.RoleBindingSpec{
				Role:     input.Role,
				Subjects: subjects,
			},
		}

		result, err := r.client.RoleBinding(input.Project).Create(roleBindingObj)
		if err != nil {
			return nil, nil, fmt.Errorf("error creating role binding '%s' in project '%s': %w", input.Name, input.Project, err)
		}

		resultJSON, err := json.Marshal(result)
		if err != nil {
			return nil, nil, fmt.Errorf("error marshalling created role binding: %w", err)
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
		ResourceType: tools.RoleBindingResource,
		RegisterWith: func(server *mcp.Server) { mcp.AddTool(server, tool, handler) },
	}
}

type UpdateProjectRoleBindingInput struct {
	Project  string   `json:"project" jsonschema:"Project name"`
	Name     string   `json:"name" jsonschema:"Role Binding name"`
	Role     string   `json:"role" jsonschema:"Name of the Role to bind"`
	Subjects []string `json:"subjects" jsonschema:"List of user names to bind to the role"`
}

func (r *roleBinding) Update() *tools.Tool {
	tool := &mcp.Tool{
		Name:        "perses_update_project_role_binding",
		Description: "Update an existing project role binding",
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
				"role": {
					Type:        "string",
					Description: "Name of the Role to bind",
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
			Required: []string{"project", "name", "role", "subjects"},
		},
		Annotations: &mcp.ToolAnnotations{
			DestructiveHint: jsonschema.Ptr(false),
			IdempotentHint:  true,
			OpenWorldHint:   jsonschema.Ptr(false),
			ReadOnlyHint:    false,
			Title:           "Updates an existing project role binding in Perses",
		},
	}

	handler := func(ctx context.Context, _ *mcp.CallToolRequest, input UpdateProjectRoleBindingInput) (*mcp.CallToolResult, any, error) {
		subjects := make([]v1.Subject, len(input.Subjects))
		for i, s := range input.Subjects {
			subjects[i] = v1.Subject{
				Kind: v1.KindUser,
				Name: s,
			}
		}

		roleBindingObj := &v1.RoleBinding{
			Kind: v1.KindRoleBinding,
			Metadata: v1.ProjectMetadata{
				Metadata: v1.Metadata{
					Name: input.Name,
				},
				ProjectMetadataWrapper: v1.ProjectMetadataWrapper{
					Project: input.Project,
				},
			},
			Spec: v1.RoleBindingSpec{
				Role:     input.Role,
				Subjects: subjects,
			},
		}

		result, err := r.client.RoleBinding(input.Project).Update(roleBindingObj)
		if err != nil {
			return nil, nil, fmt.Errorf("error updating role binding '%s' in project '%s': %w", input.Name, input.Project, err)
		}

		resultJSON, err := json.Marshal(result)
		if err != nil {
			return nil, nil, fmt.Errorf("error marshalling updated role binding: %w", err)
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
		ResourceType: tools.RoleBindingResource,
		RegisterWith: func(server *mcp.Server) { mcp.AddTool(server, tool, handler) },
	}
}

type DeleteProjectRoleBindingInput struct {
	Project string `json:"project" jsonschema:"Project name"`
	Name    string `json:"name" jsonschema:"Role Binding name to delete"`
}

func (r *roleBinding) Delete() *tools.Tool {
	tool := &mcp.Tool{
		Name:        "perses_delete_project_role_binding",
		Description: "Delete a project role binding",
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
		Annotations: &mcp.ToolAnnotations{
			DestructiveHint: jsonschema.Ptr(true),
			IdempotentHint:  true,
			OpenWorldHint:   jsonschema.Ptr(false),
			ReadOnlyHint:    false,
			Title:           "Deletes a project role binding in Perses",
		},
	}

	handler := func(ctx context.Context, _ *mcp.CallToolRequest, input DeleteProjectRoleBindingInput) (*mcp.CallToolResult, any, error) {
		err := r.client.RoleBinding(input.Project).Delete(input.Name)
		if err != nil {
			return nil, nil, fmt.Errorf("error deleting role binding '%s' in project '%s': %w", input.Name, input.Project, err)
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: fmt.Sprintf("Role binding '%s' deleted successfully from project '%s'", input.Name, input.Project),
				},
			},
		}, nil, nil
	}

	return &tools.Tool{
		MCPTool:      tool,
		IsWriteTool:  true,
		ResourceType: tools.RoleBindingResource,
		RegisterWith: func(server *mcp.Server) { mcp.AddTool(server, tool, handler) },
	}
}
