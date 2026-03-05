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

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/perses/common/async"
	"github.com/perses/common/set"
	v1 "github.com/perses/perses/pkg/client/api/v1"
	"github.com/perses/perses/pkg/client/config"
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

func New(cfg Config) (async.SimpleTask, error) {
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

	return &server{
		cfg:          cfg,
		persesClient: persesClient,
		mcpServer:    mcpServer,
	}, nil
}

func initializePersesClient(cfg Config) (v1.ClientInterface, error) {
	restClient, err := config.NewRESTClient(cfg.PersesServer)
	if err != nil {
		return nil, fmt.Errorf("error creating Perses REST client: %w", err)
	}

	return v1.NewWithClient(restClient), nil
}

type server struct {
	async.SimpleTask
	// cfg contains the server configuration settings
	cfg Config
	// persesClient is the client interface for interacting with the Perses API
	persesClient v1.ClientInterface
	// mcpServer is the Model Context Protocol server instance
	mcpServer *mcp.Server
}

func (s *server) Execute(ctx context.Context, cancelFunc context.CancelFunc) error {
	logrus.WithFields(logrus.Fields{
		"read_only": s.cfg.ReadOnly,
		"transport": s.cfg.Transport,
	}).Info("Starting Perses MCP Server")

	s.registerTools()
	// start server
	serverCtx, serverCancelFunc := context.WithCancel(ctx)
	go func() {
		defer serverCancelFunc()
		if strings.ToLower(s.cfg.Transport) == "http" {
			if err := s.runHTTPTransport(); err != nil {
				logrus.WithError(err).Info("http server stopped")
			}
		} else {
			if err := s.runStdioTransport(ctx); err != nil {
				logrus.WithError(err).Info("stdio server stopped")
			}
		}
	}()

	logrus.WithField("transport", s.cfg.Transport).Info("Perses MCP Server is running")

	// Wait for the end of the task or cancellation
	select {
	case <-serverCtx.Done():
		// Server ended unexpectedly
		// In our ecosystem, as we are producing each time an HTTP API, if the HTTP api stopped, we want to stop the whole application.
		// That's why we are calling the parent cancelFunc
		cancelFunc()
		// as it is possible that the serverCtx.Done() is closed because the main cancelFunc() has been called by another go routing,
		// we should try to close properly the http server
		// Note: that's why we don't return any error here.
	case <-ctx.Done():
		// Cancellation requested by the parent context
		logrus.Debug("server cancellation requested")
	}
	return nil
}

func (s *server) registerTools() {
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

func (s *server) runStdioTransport(ctx context.Context) error {
	logrus.Info("Running MCP server with stdio transport")
	return s.mcpServer.Run(ctx, &mcp.StdioTransport{})
}

func (s *server) runHTTPTransport() error {
	handler := mcp.NewStreamableHTTPHandler(func(req *http.Request) *mcp.Server {
		return s.mcpServer
	}, nil)

	httpServer := &http.Server{Addr: s.cfg.ListenAddress, Handler: handler}
	return httpServer.ListenAndServe()
}

func (s *server) String() string {
	return "MCP server"
}
