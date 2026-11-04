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
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/perses/mcp-server/pkg/tools"
	"github.com/perses/mcp-server/pkg/tools/validate"
	apiClient "github.com/perses/perses/pkg/client/api"
	"github.com/perses/perses/pkg/client/perseshttp"
	"github.com/perses/spec/go/common"
)

const (
	authorization         = "Bearer test-token"
	timeSeriesChartSchema = "#TimeSeriesChart: {\n\tkind: \"TimeSeriesChart\"\n}\n"
	statChartSchema       = "#StatChart: {\n\tkind: \"StatChart\"\n}\n"
	// invalidCalculationMessage is how the Perses server reports a value rejected by a plugin schema.
	invalidCalculationMessage = `bad request: invalid panel p1: spec.calculation: conflicting values "mean" and "average"`
)

const validDashboard = `{
	"kind": "Dashboard",
	"metadata": {"name": "cpu"},
	"spec": {
		"duration": "1h",
		"panels": {"p1": {"kind": "Panel", "spec": {"display": {"name": "CPU"}, "plugin": {"kind": "StatChart", "spec": {"calculation": "mean"}}}}},
		"layouts": [{"kind": "Grid", "spec": {"items": [{"x": 0, "y": 0, "width": 12, "height": 6, "content": {"$ref": "#/spec/panels/p1"}}]}}]
	}
}`

// fakePerses mimics the schema and validate APIs of the Perses server and records the requests it receives.
type fakePerses struct {
	mutex    sync.Mutex
	requests []*http.Request
	// validatedProject is the metadata.project of the last validated dashboard.
	validatedProject string
}

func (f *fakePerses) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mutex.Lock()
	f.requests = append(f.requests, r)
	f.mutex.Unlock()

	switch r.URL.Path {
	case "/api/validate/dashboards":
		f.validate(w, r)
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

// validate accepts any dashboard, except a "average" calculation that the schemas reject and a "crash" one that
// makes the server fail.
func (f *fakePerses) validate(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	var dashboard struct {
		Metadata struct {
			Project string `json:"project"`
		} `json:"metadata"`
	}
	_ = json.Unmarshal(body, &dashboard)
	f.mutex.Lock()
	f.validatedProject = dashboard.Metadata.Project
	f.mutex.Unlock()

	switch {
	case strings.Contains(string(body), `"average"`):
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"message": invalidCalculationMessage})
	case strings.Contains(string(body), `"crash"`):
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"message": "internal server error"})
	default:
		w.WriteHeader(http.StatusOK)
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

// connect registers schema and validation tools together to exercise their integration.
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
	toolset := append(New(client.V1()).GetTools(), validate.New(client).GetTools()...)
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

// validation calls perses_validate_dashboard and returns its result.
func validation(t *testing.T, session *mcp.ClientSession, dashboard any) string {
	t.Helper()
	result, text := callTool(t, session, "perses_validate_dashboard", map[string]any{"project": "myproject", "dashboard": dashboard})
	if result.IsError {
		t.Fatalf("unexpected tool error: %s", text)
	}
	return text
}

func TestValidateDashboard(t *testing.T) {
	withMetadata := func(metadata string) map[string]any {
		return dashboardObject(t, strings.Replace(validDashboard, `"name": "cpu"`, metadata, 1))
	}

	testSuites := []struct {
		title     string
		dashboard map[string]any
		expected  string
		// expectedProject is the metadata.project sent to the server; empty when the dashboard is rejected beforehand.
		expectedProject string
	}{
		{
			title:           "valid dashboard",
			dashboard:       dashboardObject(t, validDashboard),
			expected:        `{"valid":true}`,
			expectedProject: "myproject",
		},
		{
			title:           "the metadata project is kept",
			dashboard:       withMetadata(`"name": "cpu", "project": "other"`),
			expected:        `{"valid":true}`,
			expectedProject: "other",
		},
		{
			title:           "dashboard rejected by the Perses schemas",
			dashboard:       dashboardObject(t, strings.Replace(validDashboard, `"mean"`, `"average"`, 1)),
			expected:        `{"valid":false,"error":` + strconv.Quote(invalidCalculationMessage) + `}`,
			expectedProject: "myproject",
		},
		{
			title:     "not a dashboard",
			dashboard: dashboardObject(t, strings.Replace(validDashboard, `"kind": "Dashboard"`, `"kind": "EphemeralDashboard"`, 1)),
			expected:  `{"valid":false,"error":"invalid kind: \"EphemeralDashboard\" for a Dashboard type"}`,
		},
	}

	for _, test := range testSuites {
		t.Run(test.title, func(t *testing.T) {
			perses := &fakePerses{}
			if text := validation(t, connect(t, perses), test.dashboard); text != test.expected {
				t.Errorf("expected %s, got %s", test.expected, text)
			}
			if perses.validatedProject != test.expectedProject {
				t.Errorf("expected the dashboard to be validated in project %q, got %q", test.expectedProject, perses.validatedProject)
			}
		})
	}
}

func dashboardObject(t *testing.T, document string) map[string]any {
	t.Helper()
	var object map[string]any
	if err := json.Unmarshal([]byte(document), &object); err != nil {
		t.Fatal(err)
	}
	return object
}

func TestValidateDashboardServerError(t *testing.T) {
	session := connect(t, &fakePerses{})

	dashboard := dashboardObject(t, strings.Replace(validDashboard, `"mean"`, `"crash"`, 1))
	result, text := callTool(t, session, "perses_validate_dashboard", map[string]any{"project": "myproject", "dashboard": dashboard})
	if !result.IsError {
		t.Fatalf("expected a tool error when the validation cannot be performed, got %s", text)
	}
	if !strings.Contains(text, "error validating dashboard in project 'myproject'") {
		t.Errorf("unexpected error %q", text)
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

// noSchemaFound is the error returned when Perses has no schema for the kind.
func noSchemaFound(kind string) string {
	return fmt.Sprintf("no schema of plugin %q found (requires Perses v0.55 or later). Use perses_list_plugin_schemas to "+
		"find plugin kinds. Server error: something wrong happened with the request to the API.  "+
		"Message: document not found StatusCode: 404", kind)
}
