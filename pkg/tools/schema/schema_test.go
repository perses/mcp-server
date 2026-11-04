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
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/perses/mcp-server/pkg/tools"
	apiClient "github.com/perses/perses/pkg/client/api"
	"github.com/perses/perses/pkg/client/perseshttp"
	"github.com/perses/spec/go/common"
	"github.com/perses/spec/go/plugin"
)

const (
	authorization         = "Bearer test-token"
	timeSeriesChartSchema = "#TimeSeriesChart: {\n\tkind: \"TimeSeriesChart\"\n}\n"
	statChartSchema       = "#StatChart: {\n\tkind: \"StatChart\"\n}\n"
)

// fakePerses mimics the schema API of the Perses server and records the requests it receives.
type fakePerses struct {
	mutex    sync.Mutex
	requests []*http.Request
}

func (f *fakePerses) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mutex.Lock()
	f.requests = append(f.requests, r)
	f.mutex.Unlock()

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

func (f *fakePerses) received() []*http.Request {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	return append([]*http.Request(nil), f.requests...)
}

// paths returns the paths of the received requests.
func (f *fakePerses) paths() []string {
	var paths []string
	for _, request := range f.received() {
		paths = append(paths, request.URL.Path)
	}
	return paths
}

// connect registers the schema tools on an in-memory MCP server backed by perses.
func connect(t *testing.T, perses http.Handler) *mcp.ClientSession {
	t.Helper()
	httpServer := httptest.NewServer(perses)
	t.Cleanup(httpServer.Close)
	restClient := &perseshttp.RESTClient{
		BaseURL: common.MustParseURL(httpServer.URL),
		Client:  httpServer.Client(),
		Headers: map[string]string{"Authorization": authorization},
	}

	server := mcp.NewServer(&mcp.Implementation{Name: "test-server"}, nil)
	client := apiClient.NewWithClient(restClient)
	toolset := New(client.V1()).GetTools()
	names := map[string]bool{}
	for _, tool := range toolset {
		if names[tool.MCPTool.Name] {
			t.Fatalf("duplicate tool registration: %s", tool.MCPTool.Name)
		}
		names[tool.MCPTool.Name] = true
		tool.RegisterWith(server)
	}
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(context.Background(), serverTransport, nil)
	if err != nil {
		t.Fatalf("connecting the server: %v", err)
	}
	t.Cleanup(func() { _ = serverSession.Close() })

	session, err := mcp.NewClient(&mcp.Implementation{Name: "test-client"}, nil).Connect(context.Background(), clientTransport, nil)
	if err != nil {
		t.Fatalf("connecting the client: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })
	return session
}

func callTool(t *testing.T, session *mcp.ClientSession, name string, arguments map[string]any) (*mcp.CallToolResult, string) {
	t.Helper()
	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: arguments})
	if err != nil {
		t.Fatalf("calling %s: %v", name, err)
	}
	var texts []string
	for _, content := range result.Content {
		if text, ok := content.(*mcp.TextContent); ok {
			texts = append(texts, text.Text)
		}
	}
	return result, strings.Join(texts, "\n")
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
			expected:  noSchemaFound("LineChart"),
		},
		{
			title:     "unknown plugin kind in another registry names the registry",
			tool:      "perses_get_plugin_schema",
			arguments: map[string]any{"kind": "LineChart", "registry": "example.com"},
			isError:   true,
			expected:  `no plugin "LineChart" in registry "example.com"`,
		},
	}

	session := connect(t, &fakePerses{})
	for _, test := range testSuites {
		t.Run(test.title, func(t *testing.T) {
			arguments := test.arguments
			if arguments == nil {
				arguments = map[string]any{}
			}
			result, text := callTool(t, session, test.tool, arguments)
			if result.IsError != test.isError {
				t.Fatalf("expected isError=%v, got %v: %s", test.isError, result.IsError, text)
			}
			if text != test.expected {
				t.Errorf("expected %q, got %q", test.expected, text)
			}
		})
	}
}

func TestGetPluginSchemaRequest(t *testing.T) {
	perses := &fakePerses{}
	session := connect(t, perses)

	result, text := callTool(t, session, "perses_get_plugin_schema", map[string]any{"kind": "TimeSeriesChart"})
	if result.IsError {
		t.Fatalf("unexpected error: %s", text)
	}
	requests := perses.received()
	if len(requests) != 1 {
		t.Fatalf("expected 1 request, got %d", len(requests))
	}
	request := requests[0]
	if request.URL.Path != "/api/v1/schemas/plugins/TimeSeriesChart" || request.URL.RawQuery != "" {
		t.Errorf("unexpected request %s", request.URL)
	}
	if got := request.Header.Get("Authorization"); got != authorization {
		t.Errorf("expected the configured authorization header, got %q", got)
	}
}

func TestGetPluginSchemaRequestWithRegistry(t *testing.T) {
	perses := &fakePerses{}
	session := connect(t, perses)

	result, text := callTool(t, session, "perses_get_plugin_schema", map[string]any{"kind": "TimeSeriesChart", "registry": "example.com"})
	if result.IsError {
		t.Fatalf("unexpected error: %s", text)
	}
	requests := perses.received()
	if len(requests) != 1 {
		t.Fatalf("expected 1 request, got %d", len(requests))
	}
	if request := requests[0]; request.URL.Path != "/api/v1/schemas/plugins/TimeSeriesChart" || request.URL.RawQuery != "registry=example.com" {
		t.Errorf("unexpected request %s", request.URL)
	}
}

func TestGetPluginSchemaRejectsInvalidKinds(t *testing.T) {
	perses := &fakePerses{}
	session := connect(t, perses)

	for _, kind := range []string{"../projects", "Time/Series", ".", "..", ".hidden", "a b", "a?b", "a%2Fb"} {
		if result, text := callTool(t, session, "perses_get_plugin_schema", map[string]any{"kind": kind}); !result.IsError {
			t.Errorf("kind %q: expected an error, got %s", kind, text)
		}
	}
	if requests := perses.received(); len(requests) != 0 {
		t.Errorf("expected no request to Perses, got %d", len(requests))
	}
}

func TestGetPluginSchemaAcceptsHyphensAndDots(t *testing.T) {
	perses := &fakePerses{}
	session := connect(t, perses)

	callTool(t, session, "perses_get_plugin_schema", map[string]any{"kind": "my-plugin.v2"})
	if paths := perses.paths(); len(paths) != 1 || paths[0] != "/api/v1/schemas/plugins/my-plugin.v2" {
		t.Errorf("expected a request to the my-plugin.v2 schema, got %v", paths)
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

// noSchemaFound is the error returned when the default registry has no schema for the kind.
func noSchemaFound(kind string) string {
	return fmt.Sprintf("no schema of plugin %q found in registry %q; set registry for plugins installed elsewhere "+
		"(requires Perses v0.55 or later). Use perses_list_plugin_schemas to find plugin kinds. Server error: "+
		"something wrong happened with the request to the API.  Message: document not found StatusCode: 404", kind, plugin.DefaultRegistry)
}
