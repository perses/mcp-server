package client

import (
	"log/slog"

	"github.com/mark3labs/mcp-go/server"
	"github.com/perses/mcp-server/pkg/tools"

	apiClient "github.com/perses/perses/pkg/client/api/v1"
	clientConfig "github.com/perses/perses/pkg/client/config"
)

type HTTPClient struct {
	clientConfig.RestConfigClient `yaml:",inline"`
}

type Client interface {
	AddReadOnlyTools(mcpServer *server.MCPServer)
	AddWriteTools(mcpServer *server.MCPServer)
}

type Perses struct {
	api apiClient.ClientInterface
}

func NewPersesClient(conf HTTPClient) (Client, error) {
	slog.Debug("Creating Perses REST client", "url", conf.URL)

	restClient, err := clientConfig.NewRESTClient(conf.RestConfigClient)
	if err != nil {
		return nil, err
	}

	return &Perses{
		api: apiClient.NewWithClient(restClient),
	}, nil
}

func (p *Perses) AddReadOnlyTools(mcpServer *server.MCPServer) {
	c := p.api

	// Projects
	mcpServer.AddTool(tools.ListProjects(c))
	mcpServer.AddTool(tools.GetProjectByName(c))

	// Dashboards
	mcpServer.AddTool(tools.ListDashboards(c))
	mcpServer.AddTool(tools.GetDashboardByName(c))

	// Datasources
	mcpServer.AddTool(tools.ListGlobalDatasources(c))
	mcpServer.AddTool(tools.ListProjectDatasources(c))
	mcpServer.AddTool(tools.GetGlobalDatasourceByName(c))
	mcpServer.AddTool(tools.GetProjectDatasourceByName(c))

	// Roles
	mcpServer.AddTool(tools.ListGlobalRoles(c))
	mcpServer.AddTool(tools.GetGlobalRoleByName(c))
	mcpServer.AddTool(tools.ListGlobalRoleBindings(c))
	mcpServer.AddTool(tools.GetGlobalRoleBindingByName(c))
	mcpServer.AddTool(tools.ListProjectRoles(c))
	mcpServer.AddTool(tools.GetProjectRoleByName(c))
	mcpServer.AddTool(tools.ListProjectRoleBindings(c))
	mcpServer.AddTool(tools.GetProjectRoleBindingByName(c))

	// Plugins
	mcpServer.AddTool(tools.ListPlugins(c))

	// Variables
	mcpServer.AddTool(tools.ListGlobalVariables(c))
	mcpServer.AddTool(tools.GetGlobalVariableByName(c))
	mcpServer.AddTool(tools.ListProjectVariables(c))
	mcpServer.AddTool(tools.GetProjectVariableByName(c))

	slog.Debug("Read-only tools registered")
}

func (p *Perses) AddWriteTools(mcpServer *server.MCPServer) {
	c := p.api

	mcpServer.AddTool(tools.CreateProject(c))
	mcpServer.AddTool(tools.CreateDashboard(c))
	mcpServer.AddTool(tools.CreateGlobalDatasource(c))
	mcpServer.AddTool(tools.UpdateGlobalDatasource(c))
	mcpServer.AddTool(tools.CreateProjectTextVariable(c))

	slog.Debug("Write tools registered")
}
