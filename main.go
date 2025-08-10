package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/mark3labs/mcp-go/server"
	"github.com/perses/mcp-server/pkg/tools"

	apiClient "github.com/perses/perses/pkg/client/api/v1"
	"github.com/perses/perses/pkg/client/config"
	"github.com/perses/perses/pkg/model/api/v1/common"
)

var (
	persesServerURL string
	transport       string
	logLevel        string
	readOnly        bool
	port            string
)

const PERSES_TOKEN = "PERSES_TOKEN"

func init() {
	flag.StringVar(&persesServerURL, "perses-server-url", "http://localhost:8080", "The Perses backend server URL")
	flag.StringVar(&logLevel, "log-level", "info", "Log level (debug, info, warn, error)")
	flag.StringVar(&transport, "transport", "stdio", "MCP protocol currently supports 'stdio' and 'http-streamable' transport mechanisms")
	flag.StringVar(&port, "port", "8000", "Port to run the HTTP Streamable server on")
	flag.BoolVar(&readOnly, "read-only", false, "Restrict the server to read-only operations")
	flag.Parse()

	// configure logging
	logHandler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: getLogLevel(logLevel),
	})
	slog.SetDefault(slog.New(logHandler))
}

func main() {

	slog.Info("The Perses Server URL is", "url", persesServerURL)
	slog.Info("Log level set to", "level", logLevel)
	slog.Info("Transport type set to", "type", transport)

	// Initialize the Perses client
	persesClient, err := initializePersesClient(persesServerURL)
	if err != nil {
		os.Exit(1)
	}

	mcpServer := server.NewMCPServer(
		"perses-mcp",
		"0.0.1",
		server.WithToolCapabilities(true),
		server.WithLogging(),
	)

	if readOnly {
		slog.Info("Starting in READ-ONLY mode")
	} else {
		slog.Info("Starting in FULL-ACCESS mode")
	}

	addReadOnlyTools(mcpServer, persesClient)

	if !readOnly {
		addWriteTools(mcpServer, persesClient)
	}

	if err := start(mcpServer); err != nil {
		slog.Error("Error starting Perses MCP server", "error", err)
		os.Exit(1)
	}
}

func start(mcpServer *server.MCPServer) error {
	switch transport {
	case "stdio":
		return server.ServeStdio(mcpServer)
	case "streamable-http":
		streamableServer := server.NewStreamableHTTPServer(mcpServer)
		slog.Info("Server ready to accept connections", "port", port)
		return streamableServer.Start(":" + port)
	default:
		return fmt.Errorf("unsupported transport type: %s", transport)
	}
}

func addReadOnlyTools(mcpServer *server.MCPServer, persesClient apiClient.ClientInterface) {
	// Project
	mcpServer.AddTool(tools.ListProjects(persesClient))
	mcpServer.AddTool(tools.GetProjectByName(persesClient))

	// Dashboard
	mcpServer.AddTool(tools.ListDashboards(persesClient))
	mcpServer.AddTool(tools.GetDashboardByName(persesClient))

	// Datasource
	mcpServer.AddTool(tools.ListGlobalDatasources(persesClient))
	mcpServer.AddTool(tools.ListProjectDatasources(persesClient))
	mcpServer.AddTool(tools.GetGlobalDatasourceByName(persesClient))
	mcpServer.AddTool(tools.GetProjectDatasourceByName(persesClient))

	// Roles and Role Bindings
	mcpServer.AddTool(tools.ListGlobalRoles(persesClient))
	mcpServer.AddTool(tools.GetGlobalRoleByName(persesClient))
	mcpServer.AddTool(tools.ListGlobalRoleBindings(persesClient))
	mcpServer.AddTool(tools.GetGlobalRoleBindingByName(persesClient))
	mcpServer.AddTool(tools.ListProjectRoles(persesClient))
	mcpServer.AddTool(tools.GetProjectRoleByName(persesClient))
	mcpServer.AddTool(tools.ListProjectRoleBindings(persesClient))
	mcpServer.AddTool(tools.GetProjectRoleBindingByName(persesClient))

	// plugins
	mcpServer.AddTool(tools.ListPlugins(persesClient))

	//Variable
	mcpServer.AddTool(tools.ListGlobalVariables(persesClient))
	mcpServer.AddTool(tools.GetGlobalVariableByName(persesClient))
	mcpServer.AddTool(tools.ListProjectVariables(persesClient))
	mcpServer.AddTool(tools.GetProjectVariableByName(persesClient))

	slog.Debug("Added read-only tools")
}

func addWriteTools(mcpServer *server.MCPServer, persesClient apiClient.ClientInterface) {
	// Project
	mcpServer.AddTool(tools.CreateProject(persesClient))

	// Dashboard
	mcpServer.AddTool(tools.CreateDashboard(persesClient))

	// Datasource
	mcpServer.AddTool(tools.CreateGlobalDatasource(persesClient))
	mcpServer.AddTool(tools.UpdateGlobalDatasource(persesClient))

	// Variable
	mcpServer.AddTool(tools.CreateProjectTextVariable(persesClient))

	slog.Debug("Added write tools")
}

func initializePersesClient(baseURL string) (apiClient.ClientInterface, error) {

	bearerToken := os.Getenv(PERSES_TOKEN)
	if bearerToken == "" {
		slog.Error(PERSES_TOKEN + " environment variable is not set")
		return nil, fmt.Errorf(PERSES_TOKEN + " environment variable is not set")
	}

	restClient, err := config.NewRESTClient(config.RestConfigClient{
		URL: common.MustParseURL(baseURL),
		Headers: map[string]string{
			"Authorization": "Bearer " + bearerToken,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("error creating Perses Client: %v", err)
	}

	client := apiClient.NewWithClient(restClient)
	return client, nil
}

func getLogLevel(level string) slog.Level {
	switch level {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
