package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	apiClient "github.com/perses/perses/pkg/client/api/v1"
)

func ListGlobalDatasources(client apiClient.ClientInterface) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("perses_list_global_datasources",
			mcp.WithDescription("List all Perses Global Datasources")),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			globalDatasources, err := client.GlobalDatasource().List("")
			if err != nil {
				return nil, fmt.Errorf("error retrieving global datasources: %w", err)
			}

			globalDatasourcesJSON, err := json.Marshal(globalDatasources)
			if err != nil {
				return nil, fmt.Errorf("error marshalling global datasources: %w", err)
			}
			return mcp.NewToolResultText(string(globalDatasourcesJSON)), nil
		}
}

func ListDatasources(client apiClient.ClientInterface) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("perses_list_datasources",
			mcp.WithDescription("List datasources for a specific project"),
			mcp.WithString("project", mcp.Required(),
				mcp.Description("Project name"))),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			project, ok := request.Params.Arguments["project"].(string)
			if !ok {
				return mcp.NewToolResultError("invalid type for 'project', expected string"), nil
			}

			datasources, err := client.Datasource(project).List("")
			if err != nil {
				return nil, fmt.Errorf("error retrieving datasources in project '%s': %w", project, err)
			}

			datasourcesJSON, err := json.Marshal(datasources)
			if err != nil {
				return nil, fmt.Errorf("error marshalling datasources: %w", err)
			}
			return mcp.NewToolResultText(string(datasourcesJSON)), nil
		}
}
