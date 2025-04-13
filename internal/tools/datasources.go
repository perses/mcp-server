package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	apiClient "github.com/perses/perses/pkg/client/api/v1"
)

func GetGlobalDatasources(client apiClient.ClientInterface) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("perses_get_global_datasources", mcp.WithDescription("Get all Perses Global Datasources")), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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
