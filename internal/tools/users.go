package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	apiClient "github.com/perses/perses/pkg/client/api/v1"
)

func ListUsers(client apiClient.ClientInterface) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("perses_list_users", mcp.WithDescription("List all users")), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {

		users, err := client.User().List("")
		if err != nil {
			return nil, fmt.Errorf("error retrieving users: %w", err)
		}

		usersJSON, err := json.Marshal(users)
		if err != nil {
			return nil, fmt.Errorf("error marshalling users: %w", err)
		}
		return mcp.NewToolResultText(string(usersJSON)), nil
	}
}
