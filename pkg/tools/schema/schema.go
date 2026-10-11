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

package schema

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/perses/mcp-server/pkg/tools"
	"github.com/perses/mcp-server/pkg/tools/internal/proxy"
	"github.com/perses/mcp-server/pkg/tools/resource"
	apiClient "github.com/perses/perses/pkg/client/api/v1"
)

const (
	dashboardSchemaPath = "/api/v1/schemas/dashboards"
	pluginSchemasPath   = "/api/v1/schemas/plugins"
)

// kindPattern matches the valid plugin kinds. The kind becomes a path segment of the request,
// so anything else is rejected, which also prevents reaching other API paths.
var kindPattern = regexp.MustCompile(`^[A-Za-z0-9_]+$`)

type schema struct {
	// The schema API returns CUE documents, which the Perses client cannot decode, so the
	// requests are sent with the raw HTTP helper, using the same connection settings.
	http *proxy.Helper
}

func New(client apiClient.ClientInterface) resource.Resource {
	return &schema{
		http: proxy.New(client),
	}
}

func (s *schema) GetTools() []*tools.Tool {
	return []*tools.Tool{
		s.DashboardSchema(),
		s.List(),
		s.Get(),
	}
}

func (s *schema) DashboardSchema() *tools.Tool {
	tool := &mcp.Tool{
		Name: "perses_get_dashboard_schema",
		Description: "Get the CUE schema of a Perses dashboard, including the schemas of all installed plugins. " +
			"The schema is large: to write a single panel, query or variable, prefer perses_get_plugin_schema. " +
			"Requires Perses v0.55 or later.",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Gets the dashboard CUE schema from Perses",
			ReadOnlyHint:    true,
			DestructiveHint: new(false),
			IdempotentHint:  true,
			OpenWorldHint:   new(false),
		},
	}

	handler := func(ctx context.Context, _ *mcp.CallToolRequest, _ map[string]any) (*mcp.CallToolResult, any, error) { //nolint:unparam
		definition, err := s.get(ctx, dashboardSchemaPath, nil)
		if err != nil {
			return nil, nil, fmt.Errorf("error retrieving the dashboard schema: %w", err)
		}
		return textResult(definition), nil, nil
	}

	return &tools.Tool{
		MCPTool:      tool,
		IsWriteTool:  false,
		ResourceType: tools.SchemaResource,
		RegisterWith: func(server *mcp.Server) { mcp.AddTool(server, tool, handler) },
	}
}

func (s *schema) List() *tools.Tool {
	tool := &mcp.Tool{
		Name: "perses_list_plugin_schemas",
		Description: "List the CUE schemas of all plugins installed on the Perses server: panels, queries, variables " +
			"and datasources. Requires Perses v0.55 or later.",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Lists the CUE schemas of all plugins in Perses",
			ReadOnlyHint:    true,
			DestructiveHint: new(false),
			IdempotentHint:  true,
			OpenWorldHint:   new(false),
		},
	}

	handler := func(ctx context.Context, _ *mcp.CallToolRequest, _ map[string]any) (*mcp.CallToolResult, any, error) { //nolint:unparam
		definitions, err := s.get(ctx, pluginSchemasPath, nil)
		if err != nil {
			return nil, nil, fmt.Errorf("error retrieving the plugin schemas: %w", err)
		}
		return textResult(definitions), nil, nil
	}

	return &tools.Tool{
		MCPTool:      tool,
		IsWriteTool:  false,
		ResourceType: tools.SchemaResource,
		RegisterWith: func(server *mcp.Server) { mcp.AddTool(server, tool, handler) },
	}
}

type GetPluginSchemaInput struct {
	Kind     string `json:"kind" jsonschema:"Plugin kind, e.g. TimeSeriesChart"`
	Version  string `json:"version,omitempty" jsonschema:"Plugin version (default: latest)"`
	Registry string `json:"registry,omitempty" jsonschema:"Plugin registry (default: perses.dev)"`
}

func (s *schema) Get() *tools.Tool {
	tool := &mcp.Tool{
		Name: "perses_get_plugin_schema",
		Description: "Get the CUE schema of a plugin installed on the Perses server, e.g. TimeSeriesChart or " +
			"PrometheusTimeSeriesQuery. Requires Perses v0.55 or later.",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Gets the CUE schema of a plugin in Perses",
			ReadOnlyHint:    true,
			DestructiveHint: new(false),
			IdempotentHint:  true,
			OpenWorldHint:   new(false),
		},
		InputSchema: &jsonschema.Schema{
			Type: tools.SchemaTypeObject,
			Properties: map[string]*jsonschema.Schema{
				"kind": {
					Type:        tools.SchemaTypeString,
					Description: "Plugin kind, e.g. TimeSeriesChart",
					MinLength:   new(1),
					MaxLength:   new(128),
					Pattern:     kindPattern.String(),
				},
				"version": {
					Type:        tools.SchemaTypeString,
					Description: "Plugin version (default: latest)",
					MaxLength:   new(64),
				},
				"registry": {
					Type:        tools.SchemaTypeString,
					Description: "Plugin registry (default: perses.dev)",
					MaxLength:   new(253),
				},
			},
			Required: []string{"kind"},
		},
	}

	handler := func(ctx context.Context, _ *mcp.CallToolRequest, input GetPluginSchemaInput) (*mcp.CallToolResult, any, error) { //nolint:unparam
		if !kindPattern.MatchString(input.Kind) {
			return nil, nil, fmt.Errorf("invalid plugin kind %q", input.Kind)
		}
		query := map[string]string{}
		if input.Version != "" {
			query["version"] = input.Version
		}
		if input.Registry != "" {
			query["registry"] = input.Registry
		}
		definition, err := s.get(ctx, pluginSchemasPath+"/"+input.Kind, query)
		if err != nil {
			return nil, nil, fmt.Errorf("error retrieving the schema of plugin '%s': %w", input.Kind, err)
		}
		return textResult(definition), nil, nil
	}

	return &tools.Tool{
		MCPTool:      tool,
		IsWriteTool:  false,
		ResourceType: tools.SchemaResource,
		RegisterWith: func(server *mcp.Server) { mcp.AddTool(server, tool, handler) },
	}
}

// Create is not supported for schemas
func (s *schema) Create() *tools.Tool {
	return nil
}

// Update is not supported for schemas
func (s *schema) Update() *tools.Tool {
	return nil
}

// Delete is not supported for schemas
func (s *schema) Delete() *tools.Tool {
	return nil
}

// get returns the body of a successful schema API response.
func (s *schema) get(ctx context.Context, path string, query map[string]string) (string, error) {
	result, err := s.http.Do(ctx, proxy.ProxyQuery{
		Method:      http.MethodGet,
		Path:        path,
		QueryParams: query,
	})
	if err != nil {
		return "", err
	}
	if result.StatusCode != http.StatusOK {
		return "", fmt.Errorf("status %d: %s", result.StatusCode, errorMessage(result.Body))
	}
	return result.Body, nil
}

// errorMessage returns the message of a Perses API error body, or the raw body when it has none.
func errorMessage(body string) string {
	var payload struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal([]byte(body), &payload); err == nil && payload.Message != "" {
		return payload.Message
	}
	return strings.TrimSpace(body)
}

func textResult(text string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: text},
		},
	}
}
