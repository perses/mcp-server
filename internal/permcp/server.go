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
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/perses/common/async"
	"github.com/perses/common/set"
	apiclient "github.com/perses/perses/pkg/client/api"
	"github.com/perses/perses/pkg/client/config"
	"github.com/sirupsen/logrus"

	"github.com/perses/mcp-server/pkg/tools"
	"github.com/perses/mcp-server/pkg/tools/dashboard"
	"github.com/perses/mcp-server/pkg/tools/datasource"
	"github.com/perses/mcp-server/pkg/tools/ephemeraldashboard"
	"github.com/perses/mcp-server/pkg/tools/globaldatasource"
	"github.com/perses/mcp-server/pkg/tools/globalrole"
	"github.com/perses/mcp-server/pkg/tools/globalrolebinding"
	"github.com/perses/mcp-server/pkg/tools/globalvariable"
	"github.com/perses/mcp-server/pkg/tools/migrate"
	"github.com/perses/mcp-server/pkg/tools/plugin"
	"github.com/perses/mcp-server/pkg/tools/project"
	"github.com/perses/mcp-server/pkg/tools/prometheus"
	"github.com/perses/mcp-server/pkg/tools/resource"
	"github.com/perses/mcp-server/pkg/tools/role"
	"github.com/perses/mcp-server/pkg/tools/rolebinding"
	"github.com/perses/mcp-server/pkg/tools/schema"
	"github.com/perses/mcp-server/pkg/tools/validate"
	"github.com/perses/mcp-server/pkg/tools/variable"
)

func New(cfg Config) (async.SimpleTask, error) {
	persesClient, err := initializePersesClient(cfg)
	if err != nil {
		return nil, err
	}

	logrus.WithField("url", cfg.PersesServer.URL).Info("Perses client initialized")

	// The tools are selected first: the instructions are built from them, so that they never mention a missing tool.
	selection := selectTools(buildTools(persesClient), cfg)

	mcpServer := mcp.NewServer(&mcp.Implementation{
		Name:  "perses-mcp-server",
		Title: "Perses MCP Server"},
		&mcp.ServerOptions{
			Instructions: buildInstructions(selection.selected),
			Capabilities: &mcp.ServerCapabilities{
				Tools: &mcp.ToolCapabilities{ListChanged: false},
			},
		})
	mcpServer.AddReceivingMiddleware(recoverPanics)

	for _, tool := range selection.selected {
		tool.RegisterWith(mcpServer)
	}

	return &server{
		cfg:       cfg,
		mcpServer: mcpServer,
		tools:     selection,
	}, nil
}

func initializePersesClient(cfg Config) (apiclient.ClientInterface, error) {
	restClient, err := config.NewRESTClient(cfg.PersesServer)
	if err != nil {
		return nil, fmt.Errorf("error creating Perses REST client: %w", err)
	}

	return apiclient.NewWithClient(restClient), nil
}

type server struct {
	async.SimpleTask
	// cfg contains the server configuration settings
	cfg Config
	// mcpServer is the Model Context Protocol server instance
	mcpServer *mcp.Server
	// tools are the registered and the skipped tools, logged when the server starts
	tools toolSelection
}

func (s *server) Execute(ctx context.Context, cancelFunc context.CancelFunc) error {
	logrus.WithFields(logrus.Fields{
		"read_only": s.cfg.ReadOnly,
		"transport": s.cfg.Transport,
	}).Info("Starting Perses MCP Server")

	// The tools are registered by New, before the application configures the logs: they are logged here.
	s.tools.log()
	// start server
	// The channel is buffered, so the go routine is not blocked when the transport stops after the end of this method.
	transportErr := make(chan error, 1)
	go func() {
		if s.cfg.Transport == HTTPTransport {
			transportErr <- s.runHTTPTransport()
		} else {
			transportErr <- s.runStdioTransport(ctx)
		}
	}()

	logrus.WithField("transport", s.cfg.Transport).Info("Perses MCP Server is running")

	// Wait for the end of the task or cancellation
	select {
	case err := <-transportErr:
		// The transport stopped by itself (for example, the HTTP address is already in use, or the stdio client closed the connection).
		// If the MCP server stopped, we want to stop the whole application.
		// That's why we are calling the parent cancelFunc.
		// It cancels ctx, so ctx is checked before: when the application is already stopping, the error is caused by the cancellation.
		stopping := ctx.Err() != nil
		cancelFunc()
		if err != nil && !stopping {
			// The application exits with the status code 1 when the task stopping it returns an error.
			return fmt.Errorf("%s transport stopped unexpectedly: %w", s.cfg.Transport, err)
		}
	case <-ctx.Done():
		// Cancellation requested by the parent context
		logrus.Debug("server cancellation requested")
	}
	return nil
}

// buildTools returns the tools of all the resources.
func buildTools(persesClient apiclient.ClientInterface) []*tools.Tool {
	v1Client := persesClient.V1()
	resources := []resource.Resource{
		project.New(v1Client),
		dashboard.New(persesClient),
		ephemeraldashboard.New(v1Client),
		datasource.New(v1Client),
		globaldatasource.New(v1Client),
		role.New(v1Client),
		globalrole.New(v1Client),
		rolebinding.New(v1Client),
		globalrolebinding.New(v1Client),
		variable.New(v1Client),
		globalvariable.New(v1Client),
		plugin.New(v1Client),
		schema.New(v1Client),
		validate.New(persesClient),
		migrate.New(persesClient),
		prometheus.New(v1Client),
	}

	var allTools []*tools.Tool
	for _, r := range resources {
		allTools = append(allTools, r.GetTools()...)
	}
	return allTools
}

// toolSelection is the result of the read_only and resources filtering of the tools.
type toolSelection struct {
	selected        []*tools.Tool
	skippedResource []*tools.Tool
	skippedReadOnly []*tools.Tool
}

// selectTools returns the tools to register: the tools of the allowed resources (all when none is configured),
// without the write tools in read-only mode.
func selectTools(allTools []*tools.Tool, cfg Config) toolSelection {
	// Build allowed resources set for filtering
	allowedResources := set.New(cfg.AllowedResources...)
	filterByResource := len(cfg.AllowedResources) > 0

	var selection toolSelection
	for _, tool := range allTools {
		switch {
		// Skip tools not in allowed resources (if filtering is enabled)
		case filterByResource && !allowedResources.Contains(string(tool.ResourceType)):
			selection.skippedResource = append(selection.skippedResource, tool)
		// Skip write tools in read-only mode
		case cfg.ReadOnly && tool.IsWriteTool:
			selection.skippedReadOnly = append(selection.skippedReadOnly, tool)
		default:
			selection.selected = append(selection.selected, tool)
		}
	}
	return selection
}

func (t toolSelection) log() {
	for _, tool := range t.skippedResource {
		logrus.WithFields(logrus.Fields{
			"tool":         tool.MCPTool.Name,
			"resourceType": tool.ResourceType,
		}).Debug("Skipping tool which is not in allowed resources")
	}
	for _, tool := range t.skippedReadOnly {
		logrus.WithField("tool", tool.MCPTool.Name).Debug("Skipping write tool in read-only mode")
	}
	logrus.WithFields(logrus.Fields{
		"registered":       len(t.selected),
		"skipped_readonly": len(t.skippedReadOnly),
		"skipped_resource": len(t.skippedResource),
		"total":            len(t.selected) + len(t.skippedReadOnly) + len(t.skippedResource),
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

	httpServer := &http.Server{
		Addr:              s.cfg.ListenAddress,
		ReadTimeout:       30 * time.Second,
		ReadHeaderTimeout: 10 * time.Second,
		Handler:           handler,
	}
	return httpServer.ListenAndServe()
}

func (s *server) String() string {
	return "MCP server"
}
