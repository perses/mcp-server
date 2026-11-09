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
	"regexp"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/perses/mcp-server/pkg/tools"
	"github.com/perses/mcp-server/pkg/tools/resource"
	apiClient "github.com/perses/perses/pkg/client/api/v1"
	"github.com/perses/perses/pkg/client/perseshttp"
)

// kindPattern matches the valid plugin kinds, e.g. TimeSeriesChart or my-plugin.v2. The kind is a single segment of
// the request path: slashes are rejected and it can't start with a dot, so neither "." nor ".." reaches other API paths.
// The kind is passed to the client unescaped (the client escapes the path itself), so this pattern is what prevents
// path traversal: don't relax it.
var kindPattern = regexp.MustCompile(`^[A-Za-z0-9_-][A-Za-z0-9_.-]*$`)

type schema struct {
	client apiClient.ClientInterface
}

func New(client apiClient.ClientInterface) resource.Resource {
	return &schema{client: client}
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
		return tools.TextResult(string(definitions)), nil, nil
	}

	return &tools.Tool{
		MCPTool:      tool,
		IsWriteTool:  false,
		ResourceType: tools.SchemaResource,
		RegisterWith: func(server *mcp.Server) { mcp.AddTool(server, tool, handler) },
	}
}

type GetPluginSchemaInput struct {
	Kind string `json:"kind"`
}

func (s *schema) Get() *tools.Tool {
	tool := &mcp.Tool{
		Name: "perses_get_plugin_schema",
		Description: "Returns the CUE schema of one installed plugin kind, e.g. StatChart (panel) or " +
			"PrometheusLabelValuesVariable (variable). Useful when writing or fixing a plugin spec. " +
			"Kind names are the spec.plugins[].spec.name values from perses_list_plugins. Requires Perses v0.55 or later.",
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
					Description: "Plugin kind, case-sensitive, e.g. StatChart or PrometheusLabelValuesVariable.",
					MinLength:   new(1),
					MaxLength:   new(128),
					Pattern:     kindPattern.String(),
				},
			},
			Required: []string{"kind"},
		},
	}

	handler := func(_ context.Context, _ *mcp.CallToolRequest, input GetPluginSchemaInput) (*mcp.CallToolResult, any, error) { //nolint:unparam
		if !kindPattern.MatchString(input.Kind) {
			return nil, nil, fmt.Errorf("invalid plugin kind %q", input.Kind)
		}
		definition, err := s.client.Schema().Plugin(input.Kind, nil)
		if errors.Is(err, perseshttp.RequestNotFoundError) {
			return nil, nil, fmt.Errorf(
				"no schema of plugin %q found (requires Perses v0.55 or later). Use perses_list_plugin_schemas to "+
					"find plugin kinds. Server error: %w",
				input.Kind, err,
			)
		}
		if err != nil {
			return nil, nil, fmt.Errorf("error retrieving the schema of plugin '%s': %w", input.Kind, err)
		}
		return tools.TextResult(string(definition)), nil, nil
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
