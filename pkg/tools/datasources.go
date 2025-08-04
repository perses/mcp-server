package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	apiClient "github.com/perses/perses/pkg/client/api/v1"
	v1 "github.com/perses/perses/pkg/model/api/v1"
	"github.com/perses/perses/pkg/model/api/v1/common"
	"github.com/perses/perses/pkg/model/api/v1/datasource"
	"github.com/perses/perses/pkg/model/api/v1/datasource/http"
)

func ListGlobalDatasources(client apiClient.ClientInterface) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("perses_list_global_datasources",
			mcp.WithDescription("List all Perses Global Datasources")),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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

func GetGlobalDatasourceByName(client apiClient.ClientInterface) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("perses_get_global_datasource_by_name",
			mcp.WithDescription("Get a global datasource by name"),
			mcp.WithString("name", mcp.Required(),
				mcp.Description("Global Datasource name"))),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			name, err := request.RequireString("name")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			globalDatasource, err := client.GlobalDatasource().Get(name)
			if err != nil {
				return nil, fmt.Errorf("error retrieving global datasource '%s': %w", name, err)
			}

			globalDatasourceJSON, err := json.Marshal(globalDatasource)
			if err != nil {
				return nil, fmt.Errorf("error marshalling global datasource '%s': %w", name, err)
			}
			return mcp.NewToolResultText(string(globalDatasourceJSON)), nil
		}
}

func ListProjectDatasources(client apiClient.ClientInterface) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("perses_list_project_datasources",
			mcp.WithDescription("List Datasources for a specific project"),
			mcp.WithString("project", mcp.Required(),
				mcp.Description("Project name"))),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			project, err := request.RequireString("project")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			datasources, err := client.Datasource(project).List("")
			if err != nil {
				return nil, fmt.Errorf("error retrieving datasources in project '%s': %w", project, err)
			}

			datasourcesJSON, err := json.Marshal(datasources)
			if err != nil {
				return nil, fmt.Errorf("error marshalling datasources: %w", err)
			}
			return mcp.NewToolResultText(string(datasourcesJSON)), nil
		}
}

func GetProjectDatasourceByName(client apiClient.ClientInterface) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("perses_get_project_datasource_by_name",
			mcp.WithDescription("Get a datasource by name in a specific project"),
			mcp.WithString("project", mcp.Required(),
				mcp.Description("Project name")),
			mcp.WithString("name", mcp.Required(),
				mcp.Description("Datasource name"))),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			project, err := request.RequireString("project")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			name, err := request.RequireString("name")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			datasource, err := client.Datasource(project).Get(name)
			if err != nil {
				return nil, fmt.Errorf("error retrieving datasource '%s' in project '%s': %w", name, project, err)
			}

			datasourceJSON, err := json.Marshal(datasource)
			if err != nil {
				return nil, fmt.Errorf("error marshalling datasource: %w", err)
			}
			return mcp.NewToolResultText(string(datasourceJSON)), nil
		}
}

func CreateGlobalDatasource(client apiClient.ClientInterface) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("perses_create_global_datasource",
			mcp.WithDescription("Create a new Perses Global Datasource"),
			mcp.WithString("name", mcp.Required(),
				mcp.Description("Global Datasource name")),
			mcp.WithString("type", mcp.Required(),
				mcp.Description("Type of datasource"),
				mcp.Enum("PrometheusDatasource", "TempoDatasource")),
			mcp.WithString("url", mcp.Required(),
				mcp.Description("Datasource URL")),
			mcp.WithString("display_name",
				mcp.Description("Display name for the datasource (optional, defaults to name)")),
			mcp.WithString("proxy_type",
				mcp.Description("Proxy type: 'HTTPProxy' for server-side proxy, 'direct' for browser direct access (optional, defaults to HTTPProxy)")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:           "Creates a new global datasource in Perses",
				ReadOnlyHint:    ToBoolPtr(false),
				DestructiveHint: ToBoolPtr(false),
				IdempotentHint:  ToBoolPtr(true),
				OpenWorldHint:   ToBoolPtr(false),
			})),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			name, err := request.RequireString("name")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			typeStr, err := request.RequireString("type")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			urlStr, err := request.RequireString("url")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			// Parse the URL
			parsedURL, err := common.ParseURL(urlStr)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("invalid URL '%s': %v", urlStr, err)), nil
			}

			// Get optional parameters
			displayName := request.GetString("display_name", name)
			proxyType := request.GetString("proxy_type", "HTTPProxy")

			// Create the datasource spec based on proxy type
			var pluginSpec interface{}
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
					Name: name,
				},
				Spec: v1.DatasourceSpec{
					Display: &common.Display{
						Name: displayName,
					},
					Default: false, // Default to false, can be updated later
					Plugin: common.Plugin{
						Kind: typeStr,
						Spec: pluginSpec,
					},
				},
			}

			response, err := client.GlobalDatasource().Create(newGlobalDatasource)
			if err != nil {
				return nil, fmt.Errorf("error creating global datasource '%s': %w", name, err)
			}

			globalDatasourceJSON, err := json.Marshal(response)
			if err != nil {
				return nil, fmt.Errorf("error marshalling global datasource '%s': %w", name, err)
			}
			return mcp.NewToolResultText(string(globalDatasourceJSON)), nil
		}
}

func UpdateGlobalDatasource(client apiClient.ClientInterface) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("perses_update_global_datasource",
			mcp.WithDescription("Update an existing Perses Global Datasource"),
			mcp.WithString("name", mcp.Required(),
				mcp.Description("Global Datasource name")),
			mcp.WithString("type", mcp.Required(),
				mcp.Description("Type of datasource"),
				mcp.Enum("PrometheusDatasource", "TempoDatasource")),
			mcp.WithString("url", mcp.Required(),
				mcp.Description("Datasource URL")),
			mcp.WithString("display_name",
				mcp.Description("Display name for the datasource (optional, defaults to name)")),
			mcp.WithString("proxy_type",
				mcp.Description("Proxy type: 'HTTPProxy' for server-side proxy, 'direct' for browser direct access (optional, defaults to HTTPProxy)")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:           "Updates an existing global datasource in Perses",
				ReadOnlyHint:    ToBoolPtr(false),
				DestructiveHint: ToBoolPtr(false),
				IdempotentHint:  ToBoolPtr(true),
				OpenWorldHint:   ToBoolPtr(false),
			})),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			name, err := request.RequireString("name")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			typeStr, err := request.RequireString("type")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			urlStr, err := request.RequireString("url")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			// Parse the URL
			parsedURL, err := common.ParseURL(urlStr)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("invalid URL '%s': %v", urlStr, err)), nil
			}

			// Get optional parameters
			displayName := request.GetString("display_name", name)
			proxyType := request.GetString("proxy_type", "HTTPProxy")

			var pluginSpec interface{}
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
					Name: name,
				},
				Spec: v1.DatasourceSpec{
					Display: &common.Display{
						Name: displayName,
					},
					Default: false, // Default to false, can be updated later
					Plugin: common.Plugin{
						Kind: typeStr,
						Spec: pluginSpec,
					},
				},
			}

			response, err := client.GlobalDatasource().Update(updatedGlobalDatasource)
			if err != nil {
				return nil, fmt.Errorf("error updating global datasource '%s': %w", name, err)
			}

			globalDatasourceJSON, err := json.Marshal(response)
			if err != nil {
				return nil, fmt.Errorf("error marshalling global datasource '%s': %w", name, err)
			}
			return mcp.NewToolResultText(string(globalDatasourceJSON)), nil
		}
}
