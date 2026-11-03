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

// notExpandedNotice starts the schemas returned without their shared types expanded, so that the caller knows that
// the definitions of calculation, thresholds and the other shared types are not included.
const notExpandedNotice = "// Shared types such as calculation and thresholds are not expanded in this schema.\n"

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
	Kind     string `json:"kind"`
	Registry string `json:"registry,omitempty"`
}

func (s *schema) Get() *tools.Tool {
	tool := &mcp.Tool{
		Name: "perses_get_plugin_schema",
		Description: "Returns the CUE schema of one installed plugin kind, e.g. StatChart (panel) or " +
			"PrometheusLabelValuesVariable (variable). Use it before writing or fixing a plugin spec. Shared types " +
			"such as calculation and thresholds are expanded inline; when they cannot be, the response starts with a " +
			"comment saying so. Find installed kinds with perses_list_plugins. Reading CUE: 'field?:' is optional, 'a | b' lists the allowed values and '*\"x\"' marks the default. " +
			"Requires Perses v0.55 or later.",
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
					Type: tools.SchemaTypeString,
					Description: "Plugin kind, case-sensitive, e.g. StatChart or PrometheusLabelValuesVariable. " +
						"Shared types such as calculation and thresholds are not plugin kinds.",
					MinLength: new(1),
					MaxLength: new(128),
					Pattern:   kindPattern.String(),
				},
				"registry": {
					Type: tools.SchemaTypeString,
					Description: "Plugin registry. Defaults to perses.dev. Set it only for plugins installed from another " +
						"registry; their shared types are not expanded.",
					MaxLength: new(253),
				},
			},
			Required: []string{"kind"},
		},
	}

	handler := func(_ context.Context, _ *mcp.CallToolRequest, input GetPluginSchemaInput) (*mcp.CallToolResult, any, error) { //nolint:unparam
		if !kindPattern.MatchString(input.Kind) {
			return nil, nil, fmt.Errorf("invalid plugin kind %q", input.Kind)
		}
		// Only the plugins of the default registry are expanded: the dashboard schema holds one schema per kind,
		// whatever its registry.
		defaultRegistry := input.Registry == "" || input.Registry == plugin.DefaultRegistry
		if defaultRegistry {
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
		definition, err := s.client.Schema().Plugin(url.PathEscape(input.Kind), &apiClient.PluginSchemaQuery{Registry: input.Registry})
		if errors.Is(err, perseshttp.RequestNotFoundError) {
			if !defaultRegistry {
				return nil, nil, fmt.Errorf("no plugin %q in registry %q", input.Kind, input.Registry)
			}
			return nil, nil, fmt.Errorf("%q is not a plugin kind; %s", input.Kind, notPluginKindHint)
		}
		if err != nil {
			return nil, nil, fmt.Errorf("error retrieving the schema of plugin '%s': %w", input.Kind, err)
		}
		return textResult(append([]byte(notExpandedNotice), definition...)), nil, nil
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

func textResult(text []byte) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: string(text)},
		},
	}
}
