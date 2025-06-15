package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	apiClient "github.com/perses/perses/pkg/client/api/v1"
)

func ListPlugins(client apiClient.ClientInterface) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("perses_list_plugins",
			mcp.WithDescription("List all Perses Plugins")),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {

			plugins, err := client.Plugin().List()
			if err != nil {
				return nil, fmt.Errorf("error retrieving plugins: %w", err)
			}

			pluginsJSON, err := json.Marshal(plugins)
			if err != nil {
				return nil, fmt.Errorf("error marshalling plugins: %w", err)
			}
			return mcp.NewToolResultText(string(pluginsJSON)), nil
		}
}
