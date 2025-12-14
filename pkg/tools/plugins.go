package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	apiClient "github.com/perses/perses/pkg/client/api/v1"
)

func ListNewPlugins(client apiClient.ClientInterface) (*mcp.Tool, mcp.ToolHandlerFor[map[string]any, any]) {
	tool := &mcp.Tool{
		Name:        "perses_list_plugins",
		Description: "List all Perses Plugins",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Lists all plugins in Perses",
			ReadOnlyHint:    true,
			DestructiveHint: jsonschema.Ptr(false),
			IdempotentHint:  true,
			OpenWorldHint:   jsonschema.Ptr(false),
		},
	}

	handler := func(ctx context.Context, _ *mcp.CallToolRequest, input map[string]any) (*mcp.CallToolResult, any, error) {
		plugins, err := client.Plugin().List()
		if err != nil {
			return nil, nil, fmt.Errorf("error retrieving plugins: %w", err)
		}

		pluginsJSON, err := json.Marshal(plugins)
		if err != nil {
			return nil, nil, fmt.Errorf("error marshalling plugins: %w", err)
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: string(pluginsJSON),
				},
			},
		}, nil, nil
	}

	return tool, handler
}
