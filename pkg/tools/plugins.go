package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	newMcp "github.com/modelcontextprotocol/go-sdk/mcp"
	apiClient "github.com/perses/perses/pkg/client/api/v1"
)

func ListNewPlugins(client apiClient.ClientInterface) (*newMcp.Tool, newMcp.ToolHandlerFor[map[string]any, any]) {
	tool := &newMcp.Tool{
		Name: 			"perses_list_plugins",
		Description: 	"List all Perses Plugins",
		Annotations: &newMcp.ToolAnnotations{
			Title:           "Lists all plugins in Perses",
			ReadOnlyHint:    true,
			DestructiveHint: jsonschema.Ptr(false),
			IdempotentHint:  true,
			OpenWorldHint:   jsonschema.Ptr(false),
		},
	}
	
	handler := func(ctx context.Context, _ *newMcp.CallToolRequest, input map[string]any) (*newMcp.CallToolResult, any, error) {
		plugins, err := client.Plugin().List()
		if err != nil {
			return nil, nil, fmt.Errorf("error retrieving plugins: %w", err)
		}

		pluginsJSON, err := json.Marshal(plugins)
		if err != nil {
			return nil, nil, fmt.Errorf("error marshalling plugins: %w", err)
		}
		return &newMcp.CallToolResult{
			Content: []newMcp.Content{
				&newMcp.TextContent{
					Text: string(pluginsJSON),
				},
			},
		}, nil, nil
	}
	
	return tool, handler
}


func ListPlugins(client apiClient.ClientInterface) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("perses_list_plugins",
			mcp.WithDescription("List all Perses Plugins"),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:           "Lists all plugins in Perses",
				ReadOnlyHint:    ToBoolPtr(true),
				DestructiveHint: ToBoolPtr(false),
				IdempotentHint:  ToBoolPtr(true),
				OpenWorldHint:   ToBoolPtr(false),
			})),
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
