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

func ListGlobalDatasources(client apiClient.ClientInterface) (mcp.Tool, mcp.ToolHandlerFor[map[string]any, any]) {
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
		globalDatasources, err := client.GlobalDatasource().List("")
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
	return tool, handler
}

type GetGlobalDatasourceByNameInput struct {
	Name string `json:"name" jsonschema:"Global Datasource name"`
}

func GetGlobalDatasourceByName(client apiClient.ClientInterface) (mcp.Tool, mcp.ToolHandlerFor[GetGlobalDatasourceByNameInput, any]) {
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
		globalDatasource, err := client.GlobalDatasource().Get(input.Name)
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
	return tool, handler
}

type ListProjectDatasourcesInput struct {
	Project string `json:"project" jsonschema:"Project name"`
}

func ListProjectDatasources(client apiClient.ClientInterface) (mcp.Tool, mcp.ToolHandlerFor[ListProjectDatasourcesInput, any]) {
	tool := mcp.Tool{
		Name:        "perses_list_project_datasources",
		Description: "List Datasources for a specific project",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Lists datasources for a specific project in Perses",
			ReadOnlyHint:    true,
			DestructiveHint: jsonschema.Ptr(false),
			IdempotentHint:  true,
			OpenWorldHint:   jsonschema.Ptr(false),
		},
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"project": {
					Type:        "string",
					Description: "Project name",
					MinLength:   jsonschema.Ptr(1),
					MaxLength:   jsonschema.Ptr(75),
					Pattern:     "^[a-zA-Z0-9_.-]+$",
				},
			},
			Required: []string{"project"},
		},
	}

	handler := func(ctx context.Context, _ *mcp.CallToolRequest, input ListProjectDatasourcesInput) (*mcp.CallToolResult, any, error) {
		datasources, err := client.Datasource(input.Project).List("")
		if err != nil {
			return nil, nil, fmt.Errorf("error retrieving datasources in project '%s': %w", input.Project, err)
		}

		datasourcesJSON, err := json.Marshal(datasources)
		if err != nil {
			return nil, nil, fmt.Errorf("error marshalling datasources: %w", err)
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: string(datasourcesJSON),
				},
			},
		}, nil, nil
	}
	return tool, handler
}

type GetProjectDatasourceByNameInput struct {
	Project string `json:"project" jsonschema:"Project name"`
	Name    string `json:"name" jsonschema:"Datasource name"`
}

func GetProjectDatasourceByName(client apiClient.ClientInterface) (mcp.Tool, mcp.ToolHandlerFor[GetProjectDatasourceByNameInput, any]) {
	tool := mcp.Tool{
		Name:        "perses_get_project_datasource_by_name",
		Description: "Get a datasource by name in a specific project",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Gets a datasource by name in a specific project in Perses",
			ReadOnlyHint:    true,
			DestructiveHint: jsonschema.Ptr(false),
			IdempotentHint:  true,
			OpenWorldHint:   jsonschema.Ptr(false),
		},
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"project": {
					Type:        "string",
					Description: "Project name",
					MinLength:   jsonschema.Ptr(1),
					MaxLength:   jsonschema.Ptr(75),
					Pattern:     "^[a-zA-Z0-9_.-]+$",
				},
				"name": {
					Type:        "string",
					Description: "Datasource name",
					MinLength:   jsonschema.Ptr(1),
					MaxLength:   jsonschema.Ptr(75),
					Pattern:     "^[a-zA-Z0-9_.-]+$",
				},
			},
			Required: []string{"project", "name"},
		},
	}

	handler := func(ctx context.Context, _ *mcp.CallToolRequest, input GetProjectDatasourceByNameInput) (*mcp.CallToolResult, any, error) {
		datasource, err := client.Datasource(input.Project).Get(input.Name)
		if err != nil {
			return nil, nil, fmt.Errorf("error retrieving datasource '%s' in project '%s': %w", input.Name, input.Project, err)
		}

		datasourceJSON, err := json.Marshal(datasource)
		if err != nil {
			return nil, nil, fmt.Errorf("error marshalling datasource: %w", err)
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: string(datasourceJSON),
				},
			},
		}, nil, nil
	}
	return tool, handler
}

type CreateGlobalDatasourceInput struct {
	Name        string `json:"name" jsonschema:"Global Datasource name"`
	Type        string `json:"type" jsonschema:"Type of datasource"`
	URL         string `json:"url" jsonschema:"Datasource URL"`
	DisplayName string `json:"display_name,omitempty" jsonschema:"Display name for the datasource (optional, defaults to name)"`
	ProxyType   string `json:"proxy_type,omitempty" jsonschema:"Proxy type: HTTPProxy for server-side proxy, direct for browser direct access (optional, defaults to HTTPProxy)"`
}

func CreateGlobalDatasource(client apiClient.ClientInterface) (mcp.Tool, mcp.ToolHandlerFor[CreateGlobalDatasourceInput, any]) {
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

		response, err := client.GlobalDatasource().Create(newGlobalDatasource)
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
	return tool, handler
}

type UpdateGlobalDatasourceInput struct {
	Name        string `json:"name" jsonschema:"Global Datasource name"`
	Type        string `json:"type" jsonschema:"Type of datasource"`
	URL         string `json:"url" jsonschema:"Datasource URL"`
	DisplayName string `json:"display_name,omitempty" jsonschema:"Display name for the datasource (optional, defaults to name)"`
	ProxyType   string `json:"proxy_type,omitempty" jsonschema:"Proxy type: HTTPProxy for server-side proxy, direct for browser direct access (optional, defaults to HTTPProxy)"`
}

func UpdateGlobalDatasource(client apiClient.ClientInterface) (mcp.Tool, mcp.ToolHandlerFor[UpdateGlobalDatasourceInput, any]) {
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

		response, err := client.GlobalDatasource().Update(updatedGlobalDatasource)
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
	return tool, handler
}
