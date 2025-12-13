package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	apiClient "github.com/perses/perses/pkg/client/api/v1"
)

func ListGlobalRoles(client apiClient.ClientInterface) (mcp.Tool, mcp.ToolHandlerFor[map[string]any, any]) {
	tool := mcp.Tool{
		Name:        "perses_list_global_roles",
		Description: "List all Perses Global Roles",
		Annotations: &mcp.ToolAnnotations{
			Title:          "Lists all global roles in Perses",
			ReadOnlyHint:   true,
			IdempotentHint: true,
			OpenWorldHint:  jsonschema.Ptr(false),
		},
	}

	handler := func(ctx context.Context, _ *mcp.CallToolRequest, input map[string]any) (*mcp.CallToolResult, any, error) {

		globalRoles, err := client.GlobalRole().List("")
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
	return tool, handler
}

type GetGlobalRoleByNameInput struct {
	Name string `json:"name" jsonschema:"Global Role name"`
}

func GetGlobalRoleByName(client apiClient.ClientInterface) (mcp.Tool, mcp.ToolHandlerFor[GetGlobalRoleByNameInput, any]) {
	tool := mcp.Tool{
		Name:        "perses_get_global_role_by_name",
		Description: "Get a global role by name",
		Annotations: &mcp.ToolAnnotations{
			Title:          "Gets a global role by name in Perses",
			ReadOnlyHint:   true,
			IdempotentHint: true,
			OpenWorldHint:  jsonschema.Ptr(false),
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

		globalRole, err := client.GlobalRole().Get(input.Name)
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
	return tool, handler
}

func ListGlobalRoleBindings(client apiClient.ClientInterface) (mcp.Tool, mcp.ToolHandlerFor[map[string]any, any]) {
	tool := mcp.Tool{
		Name:        "perses_list_global_role_bindings",
		Description: "List all Perses Global Role Bindings",
		Annotations: &mcp.ToolAnnotations{
			Title:          "Lists all global role bindings in Perses",
			ReadOnlyHint:   true,
			IdempotentHint: true,
			OpenWorldHint:  jsonschema.Ptr(false),
		},
	}
	handler := func(ctx context.Context, _ *mcp.CallToolRequest, input map[string]any) (*mcp.CallToolResult, any, error) {

		globalRoleBindings, err := client.GlobalRoleBinding().List("")
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
	return tool, handler
}

type GetGlobalRoleBindingByNameInput struct {
	Name string `json:"name" jsonschema:"Global Role Binding name"`
}

func GetGlobalRoleBindingByName(client apiClient.ClientInterface) (mcp.Tool, mcp.ToolHandlerFor[GetGlobalRoleBindingByNameInput, any]) {
	tool := mcp.Tool{
		Name:        "perses_get_global_role_binding_by_name",
		Description: "Get a global role binding by name",
		Annotations: &mcp.ToolAnnotations{
			Title:          "Gets a global role binding by name in Perses",
			ReadOnlyHint:   true,
			IdempotentHint: true,
			OpenWorldHint:  jsonschema.Ptr(false),
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

		globalRoleBinding, err := client.GlobalRoleBinding().Get(input.Name)
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
	return tool, handler
}

type ProjectRoleInput struct {
	Project string `json:"project" jsonschema:"Project name"`
}

func ListProjectRoles(client apiClient.ClientInterface) (mcp.Tool, mcp.ToolHandlerFor[ProjectRoleInput, any]) {
	tool := mcp.Tool{
		Name:        "perses_list_project_roles",
		Description: "List Roles for a specific project",
		Annotations: &mcp.ToolAnnotations{
			Title:          "Lists roles for a specific project in Perses",
			ReadOnlyHint:   true,
			IdempotentHint: true,
			OpenWorldHint:  jsonschema.Ptr(false),
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

		roles, err := client.Role(input.Project).List("")
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
	return tool, handler
}

type GetProjectRoleByNameInput struct {
	Project string `json:"project" jsonschema:"Project name"`
	Name    string `json:"name" jsonschema:"Role name"`
}

func GetProjectRoleByName(client apiClient.ClientInterface) (mcp.Tool, mcp.ToolHandlerFor[GetProjectRoleByNameInput, any]) {
	tool := mcp.Tool{
		Name:        "perses_get_project_role_by_name",
		Description: "Get a role by name in a specific project",
		Annotations: &mcp.ToolAnnotations{
			Title:          "Gets a role by name in a specific project in Perses",
			ReadOnlyHint:   true,
			IdempotentHint: true,
			OpenWorldHint:  jsonschema.Ptr(false),
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

		role, err := client.Role(input.Project).Get(input.Name)
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
	return tool, handler
}

type ProjectRoleBindingInput struct {
	Project string `json:"project" jsonschema:"Project name"`
}

func ListProjectRoleBindings(client apiClient.ClientInterface) (mcp.Tool, mcp.ToolHandlerFor[ProjectRoleBindingInput, any]) {
	tool := mcp.Tool{
		Name:        "perses_list_project_role_bindings",
		Description: "List Role Bindings for a specific project",
		Annotations: &mcp.ToolAnnotations{
			Title:          "Lists role bindings for a specific project in Perses",
			ReadOnlyHint:   true,
			IdempotentHint: true,
			OpenWorldHint:  jsonschema.Ptr(false),
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

		roleBindings, err := client.RoleBinding(input.Project).List("")
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
	return tool, handler
}

type GetProjectRoleBindingByNameInput struct {
	Project string `json:"project" jsonschema:"Project name"`
	Name    string `json:"name" jsonschema:"Role Binding name"`
}

func GetProjectRoleBindingByName(client apiClient.ClientInterface) (mcp.Tool, mcp.ToolHandlerFor[GetProjectRoleBindingByNameInput, any]) {
	tool := mcp.Tool{
		Name:        "perses_get_project_role_binding_by_name",
		Description: "Get a role binding by name in a specific project",
		Annotations: &mcp.ToolAnnotations{
			Title:          "Gets a role binding by name in a specific project in Perses",
			ReadOnlyHint:   true,
			IdempotentHint: true,
			OpenWorldHint:  jsonschema.Ptr(false),
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

		roleBinding, err := client.RoleBinding(input.Project).Get(input.Name)
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
	return tool, handler
}
