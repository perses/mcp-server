package permcp

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/perses/mcp-server/pkg/tools"
	v1 "github.com/perses/perses/pkg/client/api/v1"
	"github.com/perses/perses/pkg/client/config"
	"github.com/perses/perses/pkg/model/api/v1/common"
)

type Config struct {
	// Version of the MCP server
	Version string

	// PersesServerURL is the URL of the Perses backend server
	PersesServerURL string

	// Token is the authentication token for the Perses server
	Token string

	// ReadOnly indicates if the server should operate in read-only mode
	ReadOnly bool

	// LogFilePath is the path to the log file (if empty, logs go to stderr)
	LogFilePath string

	// LogLevel specifies the minimum log level (debug, info, warn, error)
	LogLevel string

	// Transport mechanism for the MCP server (e.g., "stdio", "http-streamable")
	Transport string

	// Port to run the HTTP Streamable server on
	Port string
}

func Serve(ctx context.Context, cfg Config) error {
	server, err := NewServer(cfg)
	if err != nil {
		return fmt.Errorf("initialization failed: %w", err)
	}

	return server.Run(ctx)
}

type Server struct {
	// cfg contains the server configuration settings
	cfg Config
	// logger is the structured logger for the server
	logger *slog.Logger
	// persesClient is the client interface for interacting with the Perses API
	persesClient v1.ClientInterface
	// mcpServer is the Model Context Protocol server instance
	mcpServer *mcp.Server
}

func NewServer(cfg Config) (*Server, error) {
	var slogHandler slog.Handler
	var logOutput io.Writer

	if cfg.LogFilePath != "" {
		file, err := os.OpenFile(cfg.LogFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return nil, fmt.Errorf("failed to open log file: %w", err)
		}
		// defer file.Close()
		logOutput = file
	} else {
		logOutput = os.Stderr
	}

	slogHandler = slog.NewTextHandler(logOutput, &slog.HandlerOptions{
		Level: logLevel(cfg.LogLevel),
	})

	logger := slog.New(slogHandler)

	persesClient, err := initializePersesClient(cfg)
	if err != nil {
		return nil, err
	}

	logger.Info("Perses client initialized", "URL", cfg.PersesServerURL)

	mcpServer := mcp.NewServer(&mcp.Implementation{
		Name:    "perses-mcp-server",
		Title:   "Perses MCP Server",
		Version: cfg.Version},
		&mcp.ServerOptions{
			HasTools:     true,
			HasResources: false,
			HasPrompts:   false,
			Logger:       logger,
		})

	return &Server{
		cfg:          cfg,
		logger:       logger,
		persesClient: persesClient,
		mcpServer:    mcpServer,
	}, nil
}

func (s *Server) Run(ctx context.Context) error {

	ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	s.logger.Info("Starting Perses MCP Server",
		"version", s.cfg.Version,
		"read_only", s.cfg.ReadOnly,
		"transport", s.cfg.Transport,
	)

	s.registerTools()

	errChan := make(chan error, 1)

	go func() {
		var runErr error

		if strings.ToLower(s.cfg.Transport) == "http" {
			runErr = s.runHTTPTransport(ctx)
		} else {
			runErr = s.runStdioTransport(ctx)
		}
		errChan <- runErr
	}()

	s.logger.Info("Perses MCP Server is running", "transport", s.cfg.Transport)

	select {
	case <-ctx.Done():
		s.logger.Info("Shutting down signal received")
		return nil
	case err := <-errChan:
		if err != nil {
			s.logger.Error("Server stopped unexpectedly", "error", err)
			return fmt.Errorf("server error: %w", err)
		}
		return nil
	}
}

func (s *Server) registerTools() {
	registry := tools.NewToolRegistry(s.persesClient)
	allTools := registry.GetAllTools()

	registeredCount := 0
	skippedCount := 0

	for _, tool := range allTools {
		if s.cfg.ReadOnly && tool.IsWriteTool {
			s.logger.Debug("Skipping write tool in read-only mode", "tool", tool.MCPTool.Name)
			skippedCount++
			continue
		}
		tool.RegisterWith(s.mcpServer)
		registeredCount++
	}

	s.logger.Info("Tools registered successfully",
		"registered", registeredCount,
		"skipped", skippedCount,
		"total", len(allTools))
}

func (s *Server) runStdioTransport(ctx context.Context) error {
	s.logger.Info("Running MCP server with stdio transport")
	return s.mcpServer.Run(ctx, &mcp.StdioTransport{})
}

func (s *Server) runHTTPTransport(ctx context.Context) error {
	handler := mcp.NewStreamableHTTPHandler(func(req *http.Request) *mcp.Server {
		return s.mcpServer
	}, nil)

	addr := ":" + s.cfg.Port
	httpServer := &http.Server{Addr: addr, Handler: handler}

	serverErr := make(chan error, 1)
	go func() {
		s.logger.Info("Listening on HTTP", "addr", addr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErr <- err
		}
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return httpServer.Shutdown(shutdownCtx)
	case err := <-serverErr:
		return err
	}
}

func initializePersesClient(cfg Config) (v1.ClientInterface, error) {
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
	return persesClient, nil
}

func logLevel(level string) slog.Level {
	switch strings.ToLower(level) {
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
