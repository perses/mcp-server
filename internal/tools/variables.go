package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	apiClient "github.com/perses/perses/pkg/client/api/v1"
)

func ListGlobalVariables(client apiClient.ClientInterface) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("perses_list_global_variables",
			mcp.WithDescription("List all Perses Global Variables")),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {

			variables, err := client.GlobalVariable().List("")
			if err != nil {
				return nil, fmt.Errorf("error retrieving global variables: %w", err)
			}

			variablesJSON, err := json.Marshal(variables)
			if err != nil {
				return nil, fmt.Errorf("error marshalling global variables: %w", err)
			}
			return mcp.NewToolResultText(string(variablesJSON)), nil
		}
}

func ListVariables(client apiClient.ClientInterface) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("perses_list_variables",
			mcp.WithDescription("List variables for a specific project"),
			mcp.WithString("project", mcp.Required(),
				mcp.Description("Project name"))),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			project, ok := request.Params.Arguments["project"].(string)
			if !ok {
				return mcp.NewToolResultError("invalid type for 'project', expected string"), nil
			}

			variables, err := client.Variable(project).List("")
			if err != nil {
				return nil, fmt.Errorf("error retrieving variables in project '%s': %w", project, err)
			}

			variablesJSON, err := json.Marshal(variables)
			if err != nil {
				return nil, fmt.Errorf("error marshalling variables: %w", err)
			}
			return mcp.NewToolResultText(string(variablesJSON)), nil
		}
}
