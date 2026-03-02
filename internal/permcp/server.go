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
	"net/http"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/perses/common/set"
	v1 "github.com/perses/perses/pkg/client/api/v1"
	"github.com/perses/perses/pkg/client/config"
	"github.com/perses/perses/pkg/model/api/v1/common"
	"github.com/sirupsen/logrus"

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
	// Transport mechanism for the MCP server (e.g., "stdio", "http")
	Transport string `yaml:"transport,omitempty"`

	// ListenAddress is the address to listen on for HTTP transport (e.g., ":8000")
	ListenAddress string `yaml:"listen_address,omitempty"`

	// ReadOnly indicates if the server should operate in read-only mode
	ReadOnly bool `yaml:"read_only,omitempty"`

	// Resources is a comma-separated list of resources to register.
	Resources string `yaml:"resources,omitempty"`

	// AllowedResources is the normalized list of resources to register.
	AllowedResources []string `yaml:"-"`

	// PersesServer is the configuration for connecting to the Perses backend server.
	// Supports multiple authentication methods: Authorization (Bearer token),
	// OAuth, BasicAuth, K8sAuth, and NativeAuth.
	PersesServer config.RestConfigClient `yaml:"perses_server"`
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

	if c.PersesServer.URL == nil {
		c.PersesServer.URL = common.MustParseURL("http://localhost:8080")
	}

	if c.ListenAddress == "" {
		c.ListenAddress = ":8000"
	} else if !strings.Contains(c.ListenAddress, ":") {
		c.ListenAddress = ":" + c.ListenAddress
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

	persesClient, err := initializePersesClient(cfg)
	if err != nil {
		return nil, err
	}

	logrus.WithField("url", cfg.PersesServer.URL).Info("Perses client initialized")

	mcpServer := mcp.NewServer(&mcp.Implementation{
		Name:  "perses-mcp-server",
		Title: "Perses MCP Server"},
		&mcp.ServerOptions{
			HasTools:     true,
			HasResources: false,
			HasPrompts:   false,
		})

	return &Server{
		cfg:          cfg,
		persesClient: persesClient,
		mcpServer:    mcpServer,
	}, nil
}

type Server struct {
	// cfg contains the server configuration settings
	cfg Config
	// persesClient is the client interface for interacting with the Perses API
	persesClient v1.ClientInterface
	// mcpServer is the Model Context Protocol server instance
	mcpServer *mcp.Server
}

func (s *Server) Run(ctx context.Context) error {
	logrus.WithFields(logrus.Fields{
		"read_only": s.cfg.ReadOnly,
		"transport": s.cfg.Transport,
	}).Info("Starting Perses MCP Server")

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

	logrus.WithField("transport", s.cfg.Transport).Info("Perses MCP Server is running")

	select {
	case <-ctx.Done():
		logrus.Info("Shutting down signal received")
		return nil
	case err := <-errChan:
		if err != nil {
			logrus.WithError(err).Error("Server stopped unexpectedly")
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
			logrus.WithFields(logrus.Fields{
				"tool":         tool.MCPTool.Name,
				"resourceType": tool.ResourceType,
			}).Debug("Skipping tool which is not in allowed resources")
			skippedResource++
			continue
		}

		// Skip write tools in read-only mode
		if s.cfg.ReadOnly && tool.IsWriteTool {
			logrus.WithField("tool", tool.MCPTool.Name).Debug("Skipping write tool in read-only mode")
			skippedReadOnly++
			continue
		}

		tool.RegisterWith(s.mcpServer)
		registeredCount++
	}

	logrus.WithFields(logrus.Fields{
		"registered":       registeredCount,
		"skipped_readonly": skippedReadOnly,
		"skipped_resource": skippedResource,
		"total":            len(allTools),
	}).Info("Tools registered successfully")
}

func (s *Server) runStdioTransport(ctx context.Context) error {
	logrus.Info("Running MCP server with stdio transport")
	return s.mcpServer.Run(ctx, &mcp.StdioTransport{})
}

func (s *Server) runHTTPTransport(ctx context.Context) error {
	handler := mcp.NewStreamableHTTPHandler(func(req *http.Request) *mcp.Server {
		return s.mcpServer
	}, nil)

	httpServer := &http.Server{Addr: s.cfg.ListenAddress, Handler: handler}

	serverErr := make(chan error, 1)
	go func() {
		logrus.WithField("addr", s.cfg.ListenAddress).Info("Listening on HTTP")
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
	if err := cfg.PersesServer.Validate(); err != nil {
		return nil, fmt.Errorf("invalid perses client configuration: %w", err)
	}

	restClient, err := config.NewRESTClient(cfg.PersesServer)
	if err != nil {
		return nil, fmt.Errorf("error creating Perses REST client: %w", err)
	}

	return v1.NewWithClient(restClient), nil
}
