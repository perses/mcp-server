// Copyright The Perses Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package permcp

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/perses/common/set"
	v1 "github.com/perses/perses/pkg/client/api/v1"
	"github.com/perses/perses/pkg/client/config"
	"github.com/perses/perses/pkg/model/api/v1/common"

	"github.com/perses/mcp-server/pkg/tools"
	"github.com/perses/mcp-server/pkg/tools/dashboard"
	"github.com/perses/mcp-server/pkg/tools/datasource"
	"github.com/perses/mcp-server/pkg/tools/globaldatasource"
	"github.com/perses/mcp-server/pkg/tools/globalrole"
	"github.com/perses/mcp-server/pkg/tools/globalrolebinding"
	"github.com/perses/mcp-server/pkg/tools/globalvariable"
	"github.com/perses/mcp-server/pkg/tools/plugin"
	"github.com/perses/mcp-server/pkg/tools/project"
	"github.com/perses/mcp-server/pkg/tools/resource"
	"github.com/perses/mcp-server/pkg/tools/role"
	"github.com/perses/mcp-server/pkg/tools/rolebinding"
	"github.com/perses/mcp-server/pkg/tools/variable"
)

type Config struct {
	// Transport mechanism for the MCP server (e.g., "stdio", "http-streamable")
	Transport string `yaml:"transport,omitempty"`

	// PersesServerURL is the URL of the Perses backend server
	PersesServerURL string `yaml:"perses_server_url,omitempty"`

	// Token is the authentication token for the Perses server
	Token string `yaml:"-"`

	// Port to run the HTTP Streamable server on
	Port string `yaml:"port,omitempty"`

	// ReadOnly indicates if the server should operate in read-only mode
	ReadOnly bool `yaml:"read_only,omitempty"`

	// Resources is a comma-separated list of resources to register.
	Resources string `yaml:"resources,omitempty"`

	// Log contains the logger configuration.
	Log LogConfig `yaml:"log,omitempty"`

	// AllowedResources is the normalized list of resources to register.
	AllowedResources []string `yaml:"-"`
}

// LogConfig contains logger settings for the server.
type LogConfig struct {
	// Level specifies the minimum log level (debug, info, warn, error)
	Level string `yaml:"level,omitempty"`

	// FilePath is the path to the log file (if empty, logs go to stderr)
	FilePath string `yaml:"file_path,omitempty"`
}

func (c *Config) Verify() error {
	if c.Transport == "" {
		c.Transport = "stdio"
	}

	switch strings.ToLower(strings.TrimSpace(c.Transport)) {
	case "stdio":
		c.Transport = "stdio"
	case "http", "http-streamable", "streamable-http":
		c.Transport = "http"
	default:
		return fmt.Errorf("unsupported transport %q. valid values are: stdio, http", c.Transport)
	}

	if c.PersesServerURL == "" {
		c.PersesServerURL = "http://localhost:8080"
	}

	if c.Port == "" {
		c.Port = "8000"
	}

	if c.Log.Level == "" {
		c.Log.Level = "info"
	}

	c.Resources = strings.TrimSpace(c.Resources)
	c.AllowedResources = parseAllowedResources(c.Resources)

	if len(c.AllowedResources) > 0 {
		if err := c.validateAllowedResources(); err != nil {
			return err
		}
	}

	return nil
}

func (c *Config) validate() error {
	return c.Verify()
}

func (c *Config) validateAllowedResources() error {
	validSet := set.New(tools.ValidResources...)
	var invalid []string
	for _, rs := range c.AllowedResources {
		if !validSet.Contains(tools.Resource(rs)) {
			invalid = append(invalid, rs)
		}
	}

	if len(invalid) > 0 {
		validNames := make([]string, len(tools.ValidResources))
		for i, r := range tools.ValidResources {
			validNames[i] = string(r)
		}
		return fmt.Errorf("invalid resource(s): %s. Valid resources are: %s",
			strings.Join(invalid, ", "),
			strings.Join(validNames, ", "))
	}
	return nil
}

func parseAllowedResources(resources string) []string {
	if resources == "" {
		return nil
	}

	allowedResources := make([]string, 0)
	for resource := range strings.SplitSeq(resources, ",") {
		resource = strings.TrimSpace(resource)
		if resource == "" {
			continue
		}
		allowedResources = append(allowedResources, strings.ToLower(resource))
	}

	if len(allowedResources) == 0 {
		return nil
	}

	return allowedResources
}

func Serve(ctx context.Context, cfg Config) error {
	server, err := newServer(cfg)
	if err != nil {
		return fmt.Errorf("initialization failed: %w", err)
	}

	return server.Run(ctx)
}

func newServer(cfg Config) (*Server, error) {
	// Validate config
	if err := cfg.validate(); err != nil {
		return nil, err
	}

	var slogHandler slog.Handler
	var logOutput io.Writer

	if cfg.Log.FilePath != "" {
		file, err := os.OpenFile(cfg.Log.FilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return nil, fmt.Errorf("failed to open log file: %w", err)
		}
		// defer file.Close()
		logOutput = file
	} else {
		logOutput = os.Stderr
	}

	slogHandler = slog.NewTextHandler(logOutput, &slog.HandlerOptions{
		Level: logLevel(cfg.Log.Level),
	})

	logger := slog.New(slogHandler)

	persesClient, err := initializePersesClient(cfg)
	if err != nil {
		return nil, err
	}

	logger.Info("Perses client initialized", "URL", cfg.PersesServerURL)

	mcpServer := mcp.NewServer(&mcp.Implementation{
		Name:  "perses-mcp-server",
		Title: "Perses MCP Server"},
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

func (s *Server) Run(ctx context.Context) error {
	s.logger.Info("Starting Perses MCP Server",
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
	resources := []resource.Resource{
		project.New(s.persesClient),
		dashboard.New(s.persesClient),
		datasource.New(s.persesClient),
		globaldatasource.New(s.persesClient),
		role.New(s.persesClient),
		globalrole.New(s.persesClient),
		rolebinding.New(s.persesClient),
		globalrolebinding.New(s.persesClient),
		variable.New(s.persesClient),
		globalvariable.New(s.persesClient),
		plugin.New(s.persesClient),
	}

	var allTools []*tools.Tool
	for _, r := range resources {
		allTools = append(allTools, r.GetTools()...)
	}

	// Build allowed resources set for filtering
	allowedResources := set.New(s.cfg.AllowedResources...)
	filterByResource := len(s.cfg.AllowedResources) > 0

	registeredCount := 0
	skippedReadOnly := 0
	skippedResource := 0

	for _, tool := range allTools {
		// Skip tools not in allowed resources (if filtering is enabled)
		if filterByResource && !allowedResources.Contains(string(tool.ResourceType)) {
			s.logger.Debug("Skipping tool which is not in allowed resources",
				"tool", tool.MCPTool.Name,
				"resourceType", tool.ResourceType)
			skippedResource++
			continue
		}

		// Skip write tools in read-only mode
		if s.cfg.ReadOnly && tool.IsWriteTool {
			s.logger.Debug("Skipping write tool in read-only mode",
				"tool", tool.MCPTool.Name)
			skippedReadOnly++
			continue
		}

		tool.RegisterWith(s.mcpServer)
		registeredCount++
	}

	s.logger.Info("Tools registered successfully",
		"registered", registeredCount,
		"skipped_readonly", skippedReadOnly,
		"skipped_resource", skippedResource,
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
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
