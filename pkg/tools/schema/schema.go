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
	"errors"
	"fmt"
	"net/url"
	"regexp"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/perses/mcp-server/pkg/tools"
	"github.com/perses/mcp-server/pkg/tools/resource"
	apiClient "github.com/perses/perses/pkg/client/api/v1"
	"github.com/perses/perses/pkg/client/perseshttp"
	v1 "github.com/perses/perses/pkg/model/api/v1"
	"github.com/perses/spec/go/plugin"
	"github.com/sirupsen/logrus"
)

// kindPattern matches the valid plugin kinds, e.g. TimeSeriesChart or my-plugin.v2. The kind is a single segment of
// the request path: slashes are rejected and it can't start with a dot, so neither "." nor ".." reaches other API paths.
var kindPattern = regexp.MustCompile(`^[A-Za-z0-9_-][A-Za-z0-9_.-]*$`)

const notPluginKindHint = "shared types such as calculation, format and thresholds are expanded inside each " +
	"plugin's schema. Use perses_list_plugin_schemas to find plugin kinds."

type schema struct {
	client       apiClient.ClientInterface
	pluginBlocks *pluginBlocks
}

func New(client apiClient.ClientInterface) resource.Resource {
	s := &schema{client: client}
	s.pluginBlocks = newPluginBlocks(
		func() ([]byte, error) { return s.client.Schema().Dashboard() },
		func() ([]v1.PluginModule, error) { return s.client.Plugin().List() },
	)
	return s
}

func (s *schema) GetTools() []*tools.Tool {
	return []*tools.Tool{
		s.List(),
		s.Get(),
	}
}

func (s *schema) List() *tools.Tool {
	tool := &mcp.Tool{
		Name: "perses_list_plugin_schemas",
		Description: "List the CUE schemas of all plugins installed on the Perses server: panels, queries, variables, " +
			"datasources and annotations. Requires Perses v0.55 or later.",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Lists the CUE schemas of all plugins in Perses",
			ReadOnlyHint:    true,
			DestructiveHint: new(false),
			IdempotentHint:  true,
			OpenWorldHint:   new(false),
		},
	}

	handler := func(_ context.Context, _ *mcp.CallToolRequest, _ map[string]any) (*mcp.CallToolResult, any, error) { //nolint:unparam
		definitions, err := s.client.Schema().Plugins()
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
			"PrometheusTimeSeriesQuery. Shared types such as calculation, format, thresholds and the datasource " +
			"selector are expanded inline. Requires Perses v0.55 or later.",
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

	handler := func(_ context.Context, _ *mcp.CallToolRequest, input GetPluginSchemaInput) (*mcp.CallToolResult, any, error) { //nolint:unparam
		if !kindPattern.MatchString(input.Kind) {
			return nil, nil, fmt.Errorf("invalid plugin kind %q", input.Kind)
		}
		// The dashboard schema only covers the latest version of each plugin, regardless of its registry.
		if input.Version == "" && input.Registry == "" {
			block, found, err := s.pluginBlocks.get(input.Kind)
			switch {
			case err != nil:
				logrus.WithError(err).Debug("unable to expand the plugin schema, falling back to the plugin schema endpoint")
			case found:
				return textResult(block), nil, nil
			default:
				logrus.WithField("kind", input.Kind).Debug("plugin kind not found in the dashboard schema, falling back to the plugin schema endpoint")
			}
		}
		definition, err := s.client.Schema().Plugin(url.PathEscape(input.Kind), &apiClient.PluginSchemaQuery{
			Version:  input.Version,
			Registry: input.Registry,
		})
		if errors.Is(err, perseshttp.RequestNotFoundError) {
			if input.Version != "" || input.Registry != "" {
				return nil, nil, fmt.Errorf("no schema of plugin %q matches version %q and registry %q: %w",
					input.Kind, valueOrDefault(input.Version, plugin.LatestVersion), valueOrDefault(input.Registry, plugin.DefaultRegistry), err)
			}
			return nil, nil, fmt.Errorf("%q is not a plugin kind; %s", input.Kind, notPluginKindHint)
		}
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

func valueOrDefault(value, defaultValue string) string {
	if value == "" {
		return defaultValue
	}
	return value
}

func textResult(text []byte) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: string(text)},
		},
	}
}
