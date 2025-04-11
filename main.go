package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	persesClient "github.com/perses/perses/pkg/client/api/v1"
	"github.com/perses/perses/pkg/client/config"
	"github.com/perses/perses/pkg/model/api/v1/common"
)

func main() {
	var persesServerURL string
	var logLevel string

	flag.StringVar(&persesServerURL, "perses-server-url", "http://localhost:8080", "The Perses backend server URL")
	flag.StringVar(&logLevel, "log-level", "info", "Log level (debug, info, warn, error)")
	flag.Parse()

	slog.Info("The Perses Server URL is", "url", persesServerURL)
	slog.Info("Log level set to", "level", logLevel)
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})))
	if logLevel == "debug" {
		slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})))
	}

	slog.Warn("Perses MCP is in alpha, use at your own risk! ")
	// Initialize the Perses client
	persesClient := initializePersesClient(persesServerURL)
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

	slog.Info("Starting Perses MCP server using stdio transport")

	mcpServer.AddTool(getProjects(persesClient))

	if err := server.ServeStdio(mcpServer); err != nil {
		slog.Error("Error starting server", "error", err)
	}
}

func getProjects(persesClient persesClient.ClientInterface) (tool mcp.Tool, handler server.ToolHandlerFunc) {
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

	bearerToken := os.Getenv("PERSES_TOKEN")
	if bearerToken == "" {
		slog.Error("PERSES_TOKEN environment variable is not set")
		return nil
	}

	restClient, err := config.NewRESTClient(config.RestConfigClient{
		URL: common.MustParseURL(baseURL),
		Headers: map[string]string{
			"Authorization": "Bearer " + bearerToken,
		},
	})
	if err != nil {
		fmt.Println("Error creating Perses Client:", err)
		return nil
	}

	client := persesClient.NewWithClient(restClient)
	return client
}
