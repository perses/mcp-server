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
			mcp.WithDescription("List all Perses Global Roles")),
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

func ListGlobalRoleBindings(client apiClient.ClientInterface) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("perses_list_global_role_bindings",
			mcp.WithDescription("List all Perses Global Role Bindings")),
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

func ListProjectRoles(client apiClient.ClientInterface) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("perses_list_project_roles",
			mcp.WithDescription("List Roles for a specific project"),
			mcp.WithString("project", mcp.Required(),
				mcp.Description("Project name"))),
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

func ListProjectRoleBindings(client apiClient.ClientInterface) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("perses_list_project_role_bindings",
			mcp.WithDescription("List Role Bindings for a specific project"),
			mcp.WithString("project", mcp.Required(),
				mcp.Description("Project name"))),
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
