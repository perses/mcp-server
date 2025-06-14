package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	apiClient "github.com/perses/perses/pkg/client/api/v1"
)

func ListDashboards(client apiClient.ClientInterface) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("perses_list_dashboards",
			mcp.WithDescription("List dashboards for a specific project"),
			mcp.WithString("project", mcp.Required(),
				mcp.Description("Project name"))),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			project, err := request.RequireString("project")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			dashboards, err := client.Dashboard(project).List("")
			if err != nil {
				return nil, fmt.Errorf("error retrieving dashboards in project '%s': %w", project, err)
			}

			dashboardsJSON, err := json.Marshal(dashboards)
			if err != nil {
				return nil, fmt.Errorf("error marshalling dashboards: %w", err)
			}
			return mcp.NewToolResultText(string(dashboardsJSON)), nil
		}
}

func GetDashboardByName(client apiClient.ClientInterface) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("perses_get_dashboard_by_name",
			mcp.WithDescription("Get a dashboard by name in a specific project"),
			mcp.WithString("project", mcp.Required(),
				mcp.Description("Project name")),
			mcp.WithString("name", mcp.Required(),
				mcp.Description("Dashboard name"))),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			project, err := request.RequireString("project")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			name, err := request.RequireString("name")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			dashboard, err := client.Dashboard(project).Get(name)
			if err != nil {
				return nil, fmt.Errorf("error retrieving dashboard '%s' in project '%s': %w", name, project, err)
			}

			dashboardJSON, err := json.Marshal(dashboard)
			if err != nil {
				return nil, fmt.Errorf("error marshalling dashboard: %w", err)
			}
			return mcp.NewToolResultText(string(dashboardJSON)), nil
		}
}
