package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	apiClient "github.com/perses/perses/pkg/client/api/v1"
)

func ListProjects(client apiClient.ClientInterface) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("perses_list_projects",
			mcp.WithDescription("List all Perses Projects")),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {

			projects, err := client.Project().List("")
			if err != nil {
				return nil, fmt.Errorf("error retrieving projects: %w", err)
			}

			projectsJSON, err := json.Marshal(projects)
			if err != nil {
				return nil, fmt.Errorf("error marshalling projects: %w", err)
			}
			return mcp.NewToolResultText(string(projectsJSON)), nil
		}
}
