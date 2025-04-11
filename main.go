package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	persesClient "github.com/perses/perses/pkg/client/api/v1"
	"github.com/perses/perses/pkg/client/config"
	"github.com/perses/perses/pkg/model/api"
	"github.com/perses/perses/pkg/model/api/v1/common"
)

func main() {

	persesClient := initializePersesClient("http://localhost:8080")
	if persesClient == nil {
		slog.Error("Failed to initialize Perses client")
		return
	}

	mcpServer := server.NewMCPServer(
		"perses-mcp",
		"0.0.1",
		server.WithResourceCapabilities(true, true),
		server.WithLogging(),
	)

	slog.Info("Starting Grafana MCP server using stdio transport")

	mcpServer.AddTool(getDashboards(persesClient))

	if err := server.ServeStdio(mcpServer); err != nil {
		slog.Error("Error starting server", "error", err)
	}
}

func getDashboards(persesClient persesClient.ClientInterface) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("perses_get_projects", mcp.WithDescription("Get all Perses Projects")), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {

		projects, err := persesClient.Project().List("")
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

func initializePersesClient(baseURL string) persesClient.ClientInterface {

	restClient, err := config.NewRESTClient(config.RestConfigClient{
		URL: common.MustParseURL(baseURL),
		NativeAuth: &api.Auth{
			Login:    "admin",
			Password: "password",
		},
	})
	if err != nil {
		fmt.Println("Error creating Perses Client:", err)
		return nil
	}

	client := persesClient.NewWithClient(restClient)
	return client
}
