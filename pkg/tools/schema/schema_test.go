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
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/perses/mcp-server/pkg/tools"
	"github.com/perses/mcp-server/pkg/tools/internal/tooltest"
)

const (
	timeSeriesChartSchema = "#TimeSeriesChart: {\n\tkind: \"TimeSeriesChart\"\n}\n"
	statChartSchema       = "#StatChart: {\n\tkind: \"StatChart\"\n}\n"
)

// serveSchemas mimics the schema API of the Perses server.
func serveSchemas(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/api/v1/schemas/plugins":
		_, _ = w.Write([]byte(timeSeriesChartSchema + statChartSchema))
	case "/api/v1/schemas/plugins/TimeSeriesChart":
		_, _ = w.Write([]byte(timeSeriesChartSchema))
	case "/api/v1/schemas/plugins/StatChart":
		_, _ = w.Write([]byte(statChartSchema))
	default:
		kind := strings.TrimPrefix(r.URL.Path, "/api/v1/schemas/plugins/")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"message": fmt.Sprintf("document not found: plugin %q not found", kind)})
	}
}

// fakePerses returns a fake of the schema API of the Perses server recording the requests it receives.
func fakePerses() *tooltest.Perses {
	return &tooltest.Perses{Handler: http.HandlerFunc(serveSchemas)}
}

// paths returns the paths of the requests received by perses.
func paths(perses *tooltest.Perses) []string {
	var paths []string
	for _, request := range perses.Received() {
		paths = append(paths, request.Path)
	}
	return paths
}

// connect registers the schema tools on an in-memory MCP server backed by perses.
func connect(t *testing.T, perses http.Handler) *mcp.ClientSession {
	t.Helper()
	return tooltest.Connect(t, New(tooltest.NewClient(t, perses).V1()).GetTools())
}
func TestSchemaTools(t *testing.T) {
	testSuites := []struct {
		title     string
		tool      string
		arguments map[string]any
		isError   bool
		expected  string
	}{
		{
			title:    "all plugin schemas",
			tool:     "perses_list_plugin_schemas",
			expected: timeSeriesChartSchema + statChartSchema,
		},
		{
			title:     "plugin schema",
			tool:      "perses_get_plugin_schema",
			arguments: map[string]any{"kind": "TimeSeriesChart"},
			expected:  timeSeriesChartSchema,
		},
		{
			title:     "unknown plugin kind returns an error",
			tool:      "perses_get_plugin_schema",
			arguments: map[string]any{"kind": "LineChart"},
			isError:   true,
			expected: `no schema of plugin "LineChart" found (requires Perses v0.55 or later). Kind names are the ` +
				`spec.plugins[].spec.name values from perses_list_plugins`,
		},
	}

	session := connect(t, fakePerses())
	for _, test := range testSuites {
		t.Run(test.title, func(t *testing.T) {
			result, text := tooltest.CallTool(t, session, test.tool, test.arguments)
			if result.IsError != test.isError {
				t.Fatalf("expected isError=%v, got %v: %s", test.isError, result.IsError, text)
			}
			if text != test.expected {
				t.Errorf("expected %q, got %q", test.expected, text)
			}
		})
	}
}

func TestSchemaToolsServerError(t *testing.T) {
	session := connect(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	for name, arguments := range map[string]map[string]any{
		"perses_list_plugin_schemas": {},
		"perses_get_plugin_schema":   {"kind": "TimeSeriesChart"},
	} {
		if result, text := tooltest.CallTool(t, session, name, arguments); !result.IsError {
			t.Errorf("%s: expected an error, got %s", name, text)
		}
	}
}

func TestGetPluginSchemaRequest(t *testing.T) {
	perses := fakePerses()
	session := connect(t, perses)

	result, text := tooltest.CallTool(t, session, "perses_get_plugin_schema", map[string]any{"kind": "TimeSeriesChart"})
	if result.IsError {
		t.Fatalf("unexpected error: %s", text)
	}
	requests := perses.Received()
	if len(requests) != 1 {
		t.Fatalf("expected 1 request, got %d", len(requests))
	}
	request := requests[0]
	if request.Path != "/api/v1/schemas/plugins/TimeSeriesChart" || request.RawQuery != "" {
		t.Errorf("unexpected request %s?%s", request.Path, request.RawQuery)
	}
	if got := request.Authorization; got != tooltest.Authorization {
		t.Errorf("expected the configured authorization header, got %q", got)
	}
}

func TestGetPluginSchemaRejectsInvalidKinds(t *testing.T) {
	perses := fakePerses()
	session := connect(t, perses)

	for _, kind := range []string{"../projects", "Time/Series", ".", "..", ".hidden", "a b", "a?b", "a%2Fb"} {
		if result, text := tooltest.CallTool(t, session, "perses_get_plugin_schema", map[string]any{"kind": kind}); !result.IsError {
			t.Errorf("kind %q: expected an error, got %s", kind, text)
		}
	}
	if requests := perses.Received(); len(requests) != 0 {
		t.Errorf("expected no request to Perses, got %d", len(requests))
	}
}

func TestGetPluginSchemaAcceptsHyphensAndDots(t *testing.T) {
	perses := fakePerses()
	session := connect(t, perses)

	tooltest.CallTool(t, session, "perses_get_plugin_schema", map[string]any{"kind": "my-plugin.v2"})
	if got := paths(perses); len(got) != 1 || got[0] != "/api/v1/schemas/plugins/my-plugin.v2" {
		t.Errorf("expected a request to the my-plugin.v2 schema, got %v", got)
	}
}

func TestSchemaToolsAreReadOnly(t *testing.T) {
	for _, tool := range New(nil).GetTools() {
		if tool.IsWriteTool || !tool.MCPTool.Annotations.ReadOnlyHint {
			t.Errorf("%s: expected a read-only tool", tool.MCPTool.Name)
		}
		if tool.ResourceType != tools.SchemaResource {
			t.Errorf("%s: expected the %q resource type, got %q", tool.MCPTool.Name, tools.SchemaResource, tool.ResourceType)
		}
	}
}
