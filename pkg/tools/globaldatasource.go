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

package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	apiClient "github.com/perses/perses/pkg/client/api/v1"
	v1 "github.com/perses/perses/pkg/model/api/v1"
	"github.com/perses/perses/pkg/model/api/v1/common"
	"github.com/perses/perses/pkg/model/api/v1/datasource"
	"github.com/perses/perses/pkg/model/api/v1/datasource/http"
)

type GlobalDatasourceInterface interface {
	List() *Tool
	Get() *Tool
	Create() *Tool
	Update() *Tool
	GetTools() []*Tool
}

type globalDatasource struct {
	GlobalDatasourceInterface
	client apiClient.ClientInterface
}

func newGlobalDatasource(client apiClient.ClientInterface) GlobalDatasourceInterface {
	return &globalDatasource{
		client: client,
	}
}

func (g *globalDatasource) GetTools() []*Tool {
	return []*Tool{
		g.List(),
		g.Get(),
		g.Create(),
		g.Update(),
	}
}

func (g *globalDatasource) List() *Tool {
	tool := mcp.Tool{
		Name:        "perses_list_global_datasources",
		Description: "List all Perses Global Datasources",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Lists all global datasources in Perses",
			ReadOnlyHint:    true,
			DestructiveHint: jsonschema.Ptr(false),
			IdempotentHint:  true,
			OpenWorldHint:   jsonschema.Ptr(false),
		},
	}

	handler := func(ctx context.Context, _ *mcp.CallToolRequest, input map[string]any) (*mcp.CallToolResult, any, error) {
		globalDatasources, err := g.client.GlobalDatasource().List("")
		if err != nil {
			return nil, nil, fmt.Errorf("error retrieving global datasources: %w", err)
		}

		globalDatasourcesJSON, err := json.Marshal(globalDatasources)
		if err != nil {
			return nil, nil, fmt.Errorf("error marshalling global datasources: %w", err)
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: string(globalDatasourcesJSON),
				},
			},
		}, nil, nil
	}

	return &Tool{
		MCPTool:     &tool,
		RegisterWith: func(server *mcp.Server) { mcp.AddTool(server, &tool, handler) },
		IsWriteTool: false,
	}
}

type GetGlobalDatasourceByNameInput struct {
	Name string `json:"name" jsonschema:"Global Datasource name"`
}

func (g *globalDatasource) Get() *Tool {
	tool := mcp.Tool{
		Name:        "perses_get_global_datasource_by_name",
		Description: "Get a global datasource by name",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Gets a global datasource by name in Perses",
			ReadOnlyHint:    true,
			DestructiveHint: jsonschema.Ptr(false),
			IdempotentHint:  true,
			OpenWorldHint:   jsonschema.Ptr(false),
		},
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"name": {
					Type:        "string",
					Description: "Global Datasource name",
					MinLength:   jsonschema.Ptr(1),
					MaxLength:   jsonschema.Ptr(75),
					Pattern:     "^[a-zA-Z0-9_.-]+$",
				},
			},
			Required: []string{"name"},
		},
	}

	handler := func(ctx context.Context, _ *mcp.CallToolRequest, input GetGlobalDatasourceByNameInput) (*mcp.CallToolResult, any, error) {
		globalDatasource, err := g.client.GlobalDatasource().Get(input.Name)
		if err != nil {
			return nil, nil, fmt.Errorf("error retrieving global datasource '%s': %w", input.Name, err)
		}

		globalDatasourceJSON, err := json.Marshal(globalDatasource)
		if err != nil {
			return nil, nil, fmt.Errorf("error marshalling global datasource '%s': %w", input.Name, err)
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: string(globalDatasourceJSON),
				},
			},
		}, nil, nil
	}

	return &Tool{
		MCPTool:     &tool,
		RegisterWith: func(server *mcp.Server) { mcp.AddTool(server, &tool, handler) },
		IsWriteTool: false,
	}
}

type CreateGlobalDatasourceInput struct {
	Name        string `json:"name" jsonschema:"Global Datasource name"`
	Type        string `json:"type" jsonschema:"Type of datasource"`
	URL         string `json:"url" jsonschema:"Datasource URL"`
	DisplayName string `json:"display_name,omitempty" jsonschema:"Display name for the datasource (optional, defaults to name)"`
	ProxyType   string `json:"proxy_type,omitempty" jsonschema:"Proxy type: HTTPProxy for server-side proxy, direct for browser direct access (optional, defaults to HTTPProxy)"`
}

func (g *globalDatasource) Create() *Tool {
	tool := mcp.Tool{
		Name:        "perses_create_global_datasource",
		Description: "Create a new Perses Global Datasource",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Creates a new global datasource in Perses",
			ReadOnlyHint:    false,
			DestructiveHint: jsonschema.Ptr(false),
			IdempotentHint:  true,
			OpenWorldHint:   jsonschema.Ptr(false),
		},
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"name": {
					Type:        "string",
					Description: "Global Datasource name",
					MinLength:   jsonschema.Ptr(1),
					MaxLength:   jsonschema.Ptr(75),
					Pattern:     "^[a-zA-Z0-9_.-]+$",
				},
				"type": {
					Type:        "string",
					Description: "Type of datasource",
					Enum:        []any{"PrometheusDatasource", "TempoDatasource"},
				},
				"url": {
					Type:        "string",
					Description: "Datasource URL",
					MinLength:   jsonschema.Ptr(1),
				},
				"display_name": {
					Type:        "string",
					Description: "Display name for the datasource (optional, defaults to name)",
				},
				"proxy_type": {
					Type:        "string",
					Description: "Proxy type: HTTPProxy for server-side proxy, direct for browser direct access (optional, defaults to HTTPProxy)",
					Enum:        []any{"HTTPProxy", "direct"},
				},
			},
			Required: []string{"name", "type", "url"},
		},
	}

	handler := func(ctx context.Context, _ *mcp.CallToolRequest, input CreateGlobalDatasourceInput) (*mcp.CallToolResult, any, error) {
		// Parse the URL
		parsedURL, err := common.ParseURL(input.URL)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid URL '%s': %w", input.URL, err)
		}

		// Set defaults
		displayName := input.DisplayName
		if displayName == "" {
			displayName = input.Name
		}
		proxyType := input.ProxyType
		if proxyType == "" {
			proxyType = "HTTPProxy"
		}

		// Create the datasource spec based on proxy type
		var pluginSpec any
		if proxyType == "direct" {
			pluginSpec = &datasource.Prometheus{
				DirectURL: parsedURL.URL,
			}
		} else {
			// Server-side proxy (default)
			pluginSpec = &datasource.Prometheus{
				Proxy: &http.Proxy{
					Kind: "HTTPProxy",
					Spec: http.Config{
						URL: parsedURL,
					},
				},
			}
		}

		newGlobalDatasource := &v1.GlobalDatasource{
			Kind: v1.KindGlobalDatasource,
			Metadata: v1.Metadata{
				Name: input.Name,
			},
			Spec: v1.DatasourceSpec{
				Display: &common.Display{
					Name: displayName,
				},
				Default: false, // Default to false, can be updated later
				Plugin: common.Plugin{
					Kind: input.Type,
					Spec: pluginSpec,
				},
			},
		}

		response, err := g.client.GlobalDatasource().Create(newGlobalDatasource)
		if err != nil {
			return nil, nil, fmt.Errorf("error creating global datasource '%s': %w", input.Name, err)
		}

		globalDatasourceJSON, err := json.Marshal(response)
		if err != nil {
			return nil, nil, fmt.Errorf("error marshalling global datasource '%s': %w", input.Name, err)
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: string(globalDatasourceJSON),
				},
			},
		}, nil, nil
	}

	return &Tool{
		MCPTool:     &tool,
		RegisterWith: func(server *mcp.Server) { mcp.AddTool(server, &tool, handler) },
		IsWriteTool: true,
	}
}

type UpdateGlobalDatasourceInput struct {
	Name        string `json:"name" jsonschema:"Global Datasource name"`
	Type        string `json:"type" jsonschema:"Type of datasource"`
	URL         string `json:"url" jsonschema:"Datasource URL"`
	DisplayName string `json:"display_name,omitempty" jsonschema:"Display name for the datasource (optional, defaults to name)"`
	ProxyType   string `json:"proxy_type,omitempty" jsonschema:"Proxy type: HTTPProxy for server-side proxy, direct for browser direct access (optional, defaults to HTTPProxy)"`
}

func (g *globalDatasource) Update() *Tool {
	tool := mcp.Tool{
		Name:        "perses_update_global_datasource",
		Description: "Update an existing Perses Global Datasource",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Updates an existing global datasource in Perses",
			ReadOnlyHint:    false,
			DestructiveHint: jsonschema.Ptr(false),
			IdempotentHint:  true,
			OpenWorldHint:   jsonschema.Ptr(false),
		},
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"name": {
					Type:        "string",
					Description: "Global Datasource name",
					MinLength:   jsonschema.Ptr(1),
					MaxLength:   jsonschema.Ptr(75),
					Pattern:     "^[a-zA-Z0-9_.-]+$",
				},
				"type": {
					Type:        "string",
					Description: "Type of datasource",
					Enum:        []any{"PrometheusDatasource", "TempoDatasource"},
				},
				"url": {
					Type:        "string",
					Description: "Datasource URL",
					MinLength:   jsonschema.Ptr(1),
				},
				"display_name": {
					Type:        "string",
					Description: "Display name for the datasource (optional, defaults to name)",
				},
				"proxy_type": {
					Type:        "string",
					Description: "Proxy type: HTTPProxy for server-side proxy, direct for browser direct access (optional, defaults to HTTPProxy)",
					Enum:        []any{"HTTPProxy", "direct"},
				},
			},
			Required: []string{"name", "type", "url"},
		},
	}

	handler := func(ctx context.Context, _ *mcp.CallToolRequest, input UpdateGlobalDatasourceInput) (*mcp.CallToolResult, any, error) {
		// Parse the URL
		parsedURL, err := common.ParseURL(input.URL)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid URL '%s': %w", input.URL, err)
		}

		// Set defaults
		displayName := input.DisplayName
		if displayName == "" {
			displayName = input.Name
		}
		proxyType := input.ProxyType
		if proxyType == "" {
			proxyType = "HTTPProxy"
		}

		var pluginSpec any
		if proxyType == "direct" {
			pluginSpec = &datasource.Prometheus{
				DirectURL: parsedURL.URL,
			}
		} else {
			// Server-side proxy (default)
			pluginSpec = &datasource.Prometheus{
				Proxy: &http.Proxy{
					Kind: "HTTPProxy",
					Spec: http.Config{
						URL: parsedURL,
					},
				},
			}
		}

		updatedGlobalDatasource := &v1.GlobalDatasource{
			Kind: v1.KindGlobalDatasource,
			Metadata: v1.Metadata{
				Name: input.Name,
			},
			Spec: v1.DatasourceSpec{
				Display: &common.Display{
					Name: displayName,
				},
				Default: false, // Default to false, can be updated later
				Plugin: common.Plugin{
					Kind: input.Type,
					Spec: pluginSpec,
				},
			},
		}

		response, err := g.client.GlobalDatasource().Update(updatedGlobalDatasource)
		if err != nil {
			return nil, nil, fmt.Errorf("error updating global datasource '%s': %w", input.Name, err)
		}

		globalDatasourceJSON, err := json.Marshal(response)
		if err != nil {
			return nil, nil, fmt.Errorf("error marshalling global datasource '%s': %w", input.Name, err)
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: string(globalDatasourceJSON),
				},
			},
		}, nil, nil
	}

	return &Tool{
		MCPTool:     &tool,
		RegisterWith: func(server *mcp.Server) { mcp.AddTool(server, &tool, handler) },
		IsWriteTool: true,
	}
}
