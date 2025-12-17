package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/mark3labs/mcp-go/server"
	"github.com/perses/mcp-server/client"
	persesConfig "github.com/perses/perses/pkg/client/config"
	"github.com/perses/perses/pkg/model/api/v1/common"
)

var (
	persesServerURL string
	transport       string
	logLevel        string
	readOnly        bool
	port            string
	tokenURL        string
)

const PERSES_TOKEN = "PERSES_TOKEN"

func init() {
	flag.StringVar(&persesServerURL, "perses-server-url", "http://localhost:8080", "The Perses backend server URL")
	flag.StringVar(&logLevel, "log-level", "info", "Log level (debug, info, warn, error)")
	flag.StringVar(&transport, "transport", "stdio", "MCP protocol currently supports 'stdio' and 'http-streamable' transport mechanisms")
	flag.StringVar(&port, "port", "8000", "Port to run the HTTP Streamable server on")
	flag.BoolVar(&readOnly, "read-only", false, "Restrict the server to read-only operations")
	flag.StringVar(&tokenURL, "token-url", "", "OAuth token endpoint URL")
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

	// Building the REST config for the Perses client
	restCfg, err := buildRestConfig()
	if err != nil {
		slog.Error("Failed to build Perses REST config", "error", err)
		return
	}

	// Creating the Perses client
	persesClient, err := client.NewPersesClient(client.HTTPClient{
		RestConfigClient: restCfg,
	})
	if err != nil {
		os.Exit(1)
		slog.Error("Failed to create Perses client", "error", err)
		return
	}

	mcpServer := server.NewMCPServer(
		"perses-mcp",
		"0.0.1",
		server.WithToolCapabilities(true),
		server.WithLogging(),
	)

	if readOnly {
		persesClient.AddReadOnlyTools(mcpServer)
		slog.Info("Starting in READ-ONLY mode")
	} else {
		persesClient.AddWriteTools(mcpServer)
		slog.Info("Starting in FULL-ACCESS mode")
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

func buildRestConfig() (persesConfig.RestConfigClient, error) {
	cfg := persesConfig.RestConfigClient{
		URL: common.MustParseURL(persesServerURL),
	}
	if token := os.Getenv(PERSES_TOKEN); token != "" {
		cfg.Headers = map[string]string{
			"Authorization": "Bearer " + token,
		}
		return cfg, nil
	} else {
		return cfg, nil
	}
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
