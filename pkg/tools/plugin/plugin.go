// Copyright 2025 The Perses Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package plugin

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/perses/mcp-server/pkg/tools"
	"github.com/perses/mcp-server/pkg/tools/resource"
	apiClient "github.com/perses/perses/pkg/client/api/v1"
)

type plugin struct {
	client apiClient.ClientInterface
}

func New(client apiClient.ClientInterface) resource.Resource {
	return &plugin{
		client: client,
	}
}

func (p *plugin) GetTools() []*tools.Tool {
	return []*tools.Tool{
		p.List(),
	}
}

func (p *plugin) List() *tools.Tool {
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
		plugins, err := p.client.Plugin().List()
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

	return &tools.Tool{
		MCPTool:      tool,
		IsWriteTool:  false,
		ResourceType: tools.PluginResource,
		RegisterWith: func(server *mcp.Server) { mcp.AddTool(server, tool, handler) },
	}
}

// Get is not yet implemented for plugin
func (p *plugin) Get() *tools.Tool {
	return nil
}

// Create is not yet implemented for plugin
func (p *plugin) Create() *tools.Tool {
	return nil
}

// Update is not yet implemented for plugin
func (p *plugin) Update() *tools.Tool {
	return nil
}

// Delete is not yet implemented for plugin
func (p *plugin) Delete() *tools.Tool {
	return nil
}
