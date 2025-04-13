package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/ibakshay/perses-mcp/internal/tools"
	"github.com/mark3labs/mcp-go/server"

	apiClient "github.com/perses/perses/pkg/client/api/v1"
	"github.com/perses/perses/pkg/client/config"
	"github.com/perses/perses/pkg/model/api/v1/common"
)

var (
	persesServerURL string
	logLevel        string
)

const PERSES_TOKEN = "PERSES_TOKEN"

func init() {
	flag.StringVar(&persesServerURL, "perses-server-url", "http://localhost:8080", "The Perses backend server URL")
	flag.StringVar(&logLevel, "log-level", "info", "Log level (debug, info, warn, error)")
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

	// Initialize the Perses client
	persesClient, err := initializePersesClient(persesServerURL)
	if err != nil {
		os.Exit(1)
	}

	mcpServer := server.NewMCPServer(
		"perses-mcp",
		"0.0.1",
		server.WithResourceCapabilities(true, true),
		server.WithLogging(),
	)

	slog.Info("Starting Perses MCP server using stdio transport")

	mcpServer.AddTool(tools.GetProjects(persesClient))

	if err := server.ServeStdio(mcpServer); err != nil {
		slog.Error("Error starting server", "error", err)
	}
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
