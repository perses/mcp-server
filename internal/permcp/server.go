package permcp

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	v1 "github.com/perses/perses/pkg/client/api/v1"
	"github.com/perses/perses/pkg/client/config"
	"github.com/perses/perses/pkg/model/api/v1/common"
)

type MCPServerConfig struct {
	// Version of the MCP server
	Version string

	// PersesServerURL is the URL of the Perses backend server
	PersesServerURL string

	// Token is the authentication token for the Perses server
	Token string

	// ReadOnly indicates if the server should operate in read-only mode
	ReadOnly bool

	// Logger for the MCP server
	Logger *slog.Logger

	// LogFilePath is the path to the log file (if empty, logs go to stderr)
	LogFilePath string

	// log level
	LogLevel string

	// Transport mechanism for the MCP server (e.g., "stdio", "http-streamable")
	Transport string

	// Port to run the HTTP Streamable server on
	Port string
}

type Input struct {
	Name string `json:"name" jsonschema:"The name of the person"`
}

type Output struct {
	Greeting string `json:"greeting" jsonschema:"The greeting to tell to the user"`
}

func SayHi(ctx context.Context, req *mcp.CallToolRequest, input Input) (*mcp.CallToolResult, Output, error) {
	return nil, Output{Greeting: "Hi " + input.Name}, nil
}

func (cfg MCPServerConfig) RunMCPServer() error {

	// Create app context
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	var slogHandler slog.Handler
	var logOutput io.Writer

	if cfg.LogFilePath != "" {
		file, err := os.OpenFile(cfg.LogFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return fmt.Errorf("failed to open log file: %w", err)
		}

		logOutput = file
		slogHandler = slog.NewTextHandler(logOutput, &slog.HandlerOptions{
			Level: logLevel(cfg.LogLevel),
		})
	} else {
		logOutput = os.Stderr
		slogHandler = slog.NewTextHandler(logOutput, &slog.HandlerOptions{
			Level: logLevel(cfg.LogLevel),
		})
	}

	logger := slog.New(slogHandler)
	logger.Info("Starting Perses Mcp Server", "Version", cfg.Version, "PersesServerURL", cfg.PersesServerURL, "ReadOnly", cfg.ReadOnly)

	// TODO: add log configuration

	// persesClient, err := cfg.initializePersesClient()
	// if err != nil {
	// 	return err
	// }

	// Create a new MCP Server instance
	mcpServer := mcp.NewServer(&mcp.Implementation{
		Name:    "perses-mcp-server",
		Title:   "Perses MCP Server",
		Version: cfg.Version},
		&mcp.ServerOptions{
			HasTools:     true,
			HasResources: false,
			HasPrompts:   false,
		})

	mcp.AddTool(mcpServer, &mcp.Tool{Name: "greet", Description: "say hi"}, SayHi)

	if err := mcpServer.Run(ctx, &mcp.StdioTransport{}); err != nil {
		slog.Error("Failed to run MCP server", "error", err)
		return err
	}

	return nil
}

func (cfg *MCPServerConfig) initializePersesClient() (v1.ClientInterface, error) {
	restClient, err := config.NewRESTClient(config.RestConfigClient{
		URL: common.MustParseURL(cfg.PersesServerURL),
		Headers: map[string]string{
			"Authorization": "Bearer " + cfg.Token,
		},
	})
	if err != nil {
		slog.Error("Failed to create Perses client", "error", err)
		return nil, err
	}

	persesClient := v1.NewWithClient(restClient)
	return persesClient, nil
}

func logLevel(level string) slog.Level {
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
