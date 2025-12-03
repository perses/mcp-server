package permcp

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/perses/mcp-server/pkg/tools"
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

	// LogLevel specifies the minimum log level (debug, info, warn, error)
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
	} else {
		logOutput = os.Stderr
	}

	slogHandler = slog.NewTextHandler(logOutput, &slog.HandlerOptions{
		Level: logLevel(cfg.LogLevel),
	})

	logger := slog.New(slogHandler)
	cfg.Logger = logger
	logger.Info("Starting Perses Mcp Server", "Version", cfg.Version, "PersesServerURL", cfg.PersesServerURL, "ReadOnly", cfg.ReadOnly)

	persesClient, err := cfg.initializePersesClient()
	if err != nil {
		return err
	}

	projects, err := persesClient.Project().List("")
	if err != nil {
		return fmt.Errorf("error when listing projects: %w", err)
	}

	logger.Info("Successfully connected to Perses server", "projects_count", len(projects))
	// Create a new MCP Server instance
	mcpServer := mcp.NewServer(&mcp.Implementation{
		Name:    "perses-mcp-server",
		Title:   "Perses MCP Server",
		Version: cfg.Version},
		&mcp.ServerOptions{
			HasTools:     true,
			HasResources: false,
			HasPrompts:   false,
			Logger:       cfg.Logger,
		})

	if cfg.ReadOnly {
		logger.Info("Starting in READ-ONLY mode")
	}

	mcp.AddTool(mcpServer, &mcp.Tool{
		Name:        "perses_list_projects",
		Description: "List all Perses projects",
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint:   true,
			IdempotentHint: true,
		},
	}, tools.ListNewProjects(persesClient))

	errChan := make(chan error, 1)

	go func() {
		errChan <- mcpServer.Run(ctx, &mcp.StdioTransport{})
	}()

	fmt.Fprintf(os.Stderr, "Perses MCP Server running on stdio\n")

	select {
	case <-ctx.Done():
		logger.Info("shutting down server", "signal", "context done")
	case err := <-errChan:
		if err != nil {
			logger.Error("error running Perses MCP Server", "error", err)
			return fmt.Errorf("error running Perses MCP Server: %w", err)
		}
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
		return nil, err
	}

	persesClient := v1.NewWithClient(restClient)
	cfg.Logger.Info("Perses client initialized", "URL", cfg.PersesServerURL)
	return persesClient, nil
}

func logLevel(level string) slog.Level {
	switch level {
	case strings.ToLower("debug"):
		return slog.LevelDebug
	case strings.ToLower("info"):
		return slog.LevelInfo
	case strings.ToLower("warn"):
		return slog.LevelWarn
	case strings.ToLower("error"):
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
