package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	apiClient "github.com/perses/perses/pkg/client/api/v1"
)

func ListGlobalRoles(client apiClient.ClientInterface) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("perses_list_global_roles",
			mcp.WithDescription("List all Perses Global Roles"),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:           "Lists all global roles in Perses",
				ReadOnlyHint:    ToBoolPtr(true),
				DestructiveHint: ToBoolPtr(false),
				IdempotentHint:  ToBoolPtr(true),
				OpenWorldHint:   ToBoolPtr(false),
			})),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {

			globalRoles, err := client.GlobalRole().List("")
			if err != nil {
				return nil, fmt.Errorf("error retrieving global roles: %w", err)
			}

			globalRolesJSON, err := json.Marshal(globalRoles)
			if err != nil {
				return nil, fmt.Errorf("error marshalling global roles: %w", err)
			}
			return mcp.NewToolResultText(string(globalRolesJSON)), nil
		}
}

func GetGlobalRoleByName(client apiClient.ClientInterface) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("perses_get_global_role_by_name",
			mcp.WithDescription("Get a global role by name"),
			mcp.WithString("name", mcp.Required(),
				mcp.Description("Global Role name")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:           "Gets a global role by name in Perses",
				ReadOnlyHint:    ToBoolPtr(true),
				DestructiveHint: ToBoolPtr(false),
				IdempotentHint:  ToBoolPtr(true),
				OpenWorldHint:   ToBoolPtr(false),
			})),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			name, err := request.RequireString("name")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			globalRole, err := client.GlobalRole().Get(name)
			if err != nil {
				return nil, fmt.Errorf("error retrieving global role '%s': %w", name, err)
			}

			globalRoleJSON, err := json.Marshal(globalRole)
			if err != nil {
				return nil, fmt.Errorf("error marshalling global role '%s': %w", name, err)
			}
			return mcp.NewToolResultText(string(globalRoleJSON)), nil
		}
}

func ListGlobalRoleBindings(client apiClient.ClientInterface) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("perses_list_global_role_bindings",
			mcp.WithDescription("List all Perses Global Role Bindings"),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:           "Lists all global role bindings in Perses",
				ReadOnlyHint:    ToBoolPtr(true),
				DestructiveHint: ToBoolPtr(false),
				IdempotentHint:  ToBoolPtr(true),
				OpenWorldHint:   ToBoolPtr(false),
			})),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {

			globalRoleBindings, err := client.GlobalRoleBinding().List("")
			if err != nil {
				return nil, fmt.Errorf("error retrieving global role bindings: %w", err)
			}

			globalRoleBindingsJSON, err := json.Marshal(globalRoleBindings)
			if err != nil {
				return nil, fmt.Errorf("error marshalling global role bindings: %w", err)
			}
			return mcp.NewToolResultText(string(globalRoleBindingsJSON)), nil
		}
}

func GetGlobalRoleBindingByName(client apiClient.ClientInterface) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("perses_get_global_role_binding_by_name",
			mcp.WithDescription("Get a global role binding by name"),
			mcp.WithString("name", mcp.Required(),
				mcp.Description("Global Role Binding name")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:           "Gets a global role binding by name in Perses",
				ReadOnlyHint:    ToBoolPtr(true),
				DestructiveHint: ToBoolPtr(false),
				IdempotentHint:  ToBoolPtr(true),
				OpenWorldHint:   ToBoolPtr(false),
			})),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			name, err := request.RequireString("name")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			globalRoleBinding, err := client.GlobalRoleBinding().Get(name)
			if err != nil {
				return nil, fmt.Errorf("error retrieving global role binding '%s': %w", name, err)
			}

			globalRoleBindingJSON, err := json.Marshal(globalRoleBinding)
			if err != nil {
				return nil, fmt.Errorf("error marshalling global role binding '%s': %w", name, err)
			}
			return mcp.NewToolResultText(string(globalRoleBindingJSON)), nil
		}
}

func ListProjectRoles(client apiClient.ClientInterface) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("perses_list_project_roles",
			mcp.WithDescription("List Roles for a specific project"),
			mcp.WithString("project", mcp.Required(),
				mcp.Description("Project name")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:           "Lists roles for a specific project in Perses",
				ReadOnlyHint:    ToBoolPtr(true),
				DestructiveHint: ToBoolPtr(false),
				IdempotentHint:  ToBoolPtr(true),
				OpenWorldHint:   ToBoolPtr(false),
			})),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			project, err := request.RequireString("project")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			roles, err := client.Role(project).List("")
			if err != nil {
				return nil, fmt.Errorf("error retrieving roles in project '%s': %w", project, err)
			}

			rolesJSON, err := json.Marshal(roles)
			if err != nil {
				return nil, fmt.Errorf("error marshalling roles: %w", err)
			}
			return mcp.NewToolResultText(string(rolesJSON)), nil
		}
}

func GetProjectRoleByName(client apiClient.ClientInterface) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("perses_get_project_role_by_name",
			mcp.WithDescription("Get a role by name in a specific project"),
			mcp.WithString("project", mcp.Required(),
				mcp.Description("Project name")),
			mcp.WithString("name", mcp.Required(),
				mcp.Description("Role name")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:           "Gets a role by name in a specific project in Perses",
				ReadOnlyHint:    ToBoolPtr(true),
				DestructiveHint: ToBoolPtr(false),
				IdempotentHint:  ToBoolPtr(true),
				OpenWorldHint:   ToBoolPtr(false),
			})),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			project, err := request.RequireString("project")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			name, err := request.RequireString("name")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			role, err := client.Role(project).Get(name)
			if err != nil {
				return nil, fmt.Errorf("error retrieving role '%s' in project '%s': %w", name, project, err)
			}

			roleJSON, err := json.Marshal(role)
			if err != nil {
				return nil, fmt.Errorf("error marshalling role: %w", err)
			}
			return mcp.NewToolResultText(string(roleJSON)), nil
		}
}

func ListProjectRoleBindings(client apiClient.ClientInterface) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("perses_list_project_role_bindings",
			mcp.WithDescription("List Role Bindings for a specific project"),
			mcp.WithString("project", mcp.Required(),
				mcp.Description("Project name")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:           "Lists role bindings for a specific project in Perses",
				ReadOnlyHint:    ToBoolPtr(true),
				DestructiveHint: ToBoolPtr(false),
				IdempotentHint:  ToBoolPtr(true),
				OpenWorldHint:   ToBoolPtr(false),
			})),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			project, err := request.RequireString("project")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			roleBindings, err := client.RoleBinding(project).List("")
			if err != nil {
				return nil, fmt.Errorf("error retrieving role bindings in project '%s': %w", project, err)
			}

			roleBindingsJSON, err := json.Marshal(roleBindings)
			if err != nil {
				return nil, fmt.Errorf("error marshalling role bindings: %w", err)
			}
			return mcp.NewToolResultText(string(roleBindingsJSON)), nil
		}
}

func GetProjectRoleBindingByName(client apiClient.ClientInterface) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("perses_get_project_role_binding_by_name",
			mcp.WithDescription("Get a role binding by name in a specific project"),
			mcp.WithString("project", mcp.Required(),
				mcp.Description("Project name")),
			mcp.WithString("name", mcp.Required(),
				mcp.Description("Role Binding name")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:           "Gets a role binding by name in a specific project in Perses",
				ReadOnlyHint:    ToBoolPtr(true),
				DestructiveHint: ToBoolPtr(false),
				IdempotentHint:  ToBoolPtr(true),
				OpenWorldHint:   ToBoolPtr(false),
			})),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			project, err := request.RequireString("project")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			name, err := request.RequireString("name")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			roleBinding, err := client.RoleBinding(project).Get(name)
			if err != nil {
				return nil, fmt.Errorf("error retrieving role binding '%s' in project '%s': %w", name, project, err)
			}

			roleBindingJSON, err := json.Marshal(roleBinding)
			if err != nil {
				return nil, fmt.Errorf("error marshalling role binding: %w", err)
			}
			return mcp.NewToolResultText(string(roleBindingJSON)), nil
		}
}
