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
	"strings"
	"sync"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/perses/mcp-server/pkg/tools"
	"github.com/perses/mcp-server/pkg/tools/internal/dashboardutil"
	"github.com/perses/mcp-server/pkg/tools/validate"
	apiClient "github.com/perses/perses/pkg/client/api"
	"github.com/perses/perses/pkg/client/perseshttp"
	"github.com/perses/spec/go/common"
)

const (
	authorization         = "Bearer test-token"
	timeSeriesChartSchema = "#TimeSeriesChart: {\n\tkind: \"TimeSeriesChart\"\n}\n"
	statChartSchema       = "#StatChart: {\n\tkind: \"StatChart\"\n}\n"
	// defaultPluginModules mimics /api/v1/plugins: the plugins of sampleDashboardSchema, in the default registry.
	defaultPluginModules = `[
	{"kind": "PluginModule", "metadata": {"name": "StatChart", "version": "0.15.0", "registry": ""},
	 "spec": {"schemasPath": "schemas", "plugins": [{"kind": "Panel", "spec": {"display": {"name": "Stat Chart"}, "name": "StatChart"}}]}},
	{"kind": "PluginModule", "metadata": {"name": "Prometheus", "version": "0.59.0", "registry": "perses.dev"},
	 "spec": {"schemasPath": "schemas", "plugins": [{"kind": "TimeSeriesQuery", "spec": {"display": {"name": "Prometheus Time Series Query"}, "name": "PrometheusTimeSeriesQuery"}}]}}
]`
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
	// dashboardSchema is served by the dashboard schema endpoint, which returns a server error when it's empty.
	dashboardSchema string
	// pluginModules is served by the plugin list endpoint, defaultPluginModules when it's empty. "error" returns a
	// server error.
	pluginModules string
	mutex         sync.Mutex
	requests      []*http.Request
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
	case "/api/v1/schemas/dashboards":
		if f.dashboardSchema == "" {
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]string{"message": "internal server error"})
			return
		}
		_, _ = w.Write([]byte(f.dashboardSchema))
	case "/api/v1/plugins":
		switch f.pluginModules {
		case "":
			_, _ = w.Write([]byte(defaultPluginModules))
		case "error":
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]string{"message": "internal server error"})
		default:
			_, _ = w.Write([]byte(f.pluginModules))
		}
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
	session, _ := connectSchema(t, perses)
	return session
}

// connectSchema is connect that also returns the schema resource, to control its cache.
func connectSchema(t *testing.T, perses http.Handler) (*mcp.ClientSession, *schema) {
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
	schemaResource := New(client.V1()).(*schema)
	toolset := append(schemaResource.GetTools(), validate.New(client).GetTools()...)
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
	return session, schemaResource
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
			expected:  notExpandedNotice + timeSeriesChartSchema,
		},
		{
			title:     "unknown plugin kind returns an error",
			tool:      "perses_get_plugin_schema",
			arguments: map[string]any{"kind": "LineChart"},
			isError:   true,
			expected:  `"LineChart" is not a plugin kind; ` + notPluginKindHint,
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
	if len(requests) == 0 {
		t.Fatal("expected a request to Perses")
	}
	request := requests[len(requests)-1]
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
	if paths := perses.paths(); len(paths) == 0 || paths[len(paths)-1] != "/api/v1/schemas/plugins/my-plugin.v2" {
		t.Errorf("expected a request to the my-plugin.v2 schema, got %v", paths)
	}
}

// validation calls perses_validate_dashboard and decodes its result.
func validation(t *testing.T, session *mcp.ClientSession, dashboard any) dashboardutil.Validation {
	t.Helper()
	result, text := callTool(t, session, "perses_validate_dashboard", map[string]any{"project": "myproject", "dashboard": dashboard})
	if result.IsError {
		t.Fatalf("unexpected tool error: %s", text)
	}
	var validation dashboardutil.Validation
	if err := json.Unmarshal([]byte(text), &validation); err != nil {
		t.Fatalf("unable to decode the validation %q: %v", text, err)
	}
	return validation
}

func TestValidateDashboard(t *testing.T) {
	var dashboardObject map[string]any
	if err := json.Unmarshal([]byte(validDashboard), &dashboardObject); err != nil {
		t.Fatal(err)
	}

	testSuites := []struct {
		title     string
		dashboard any
		// expectedPath and expectedMessage describe the single expected error; an empty message expects a valid result.
		expectedPath    string
		expectedMessage string
		// expectedProject is the metadata.project sent to the server; empty when the dashboard is rejected beforehand.
		expectedProject string
	}{
		{
			title:           "valid dashboard as a string",
			dashboard:       validDashboard,
			expectedProject: "myproject",
		},
		{
			title:           "valid dashboard as an object",
			dashboard:       dashboardObject,
			expectedProject: "myproject",
		},
		{
			title:           "the metadata project matching the project ignoring case is kept",
			dashboard:       strings.Replace(validDashboard, `"name": "cpu"`, `"name": "cpu", "project": "MyProject"`, 1),
			expectedProject: "MyProject",
		},
		{
			title:           "dashboard rejected by the Perses schemas",
			dashboard:       strings.Replace(validDashboard, `"mean"`, `"average"`, 1),
			expectedPath:    "spec.panels.p1.spec.plugin.spec.calculation",
			expectedMessage: `conflicting values "mean" and "average"`,
			expectedProject: "myproject",
		},
		{
			title:           "malformed JSON",
			dashboard:       `{"kind": "Dashboard",`,
			expectedMessage: "malformed JSON",
		},
		{
			title:           "not a dashboard",
			dashboard:       strings.Replace(validDashboard, `"kind": "Dashboard"`, `"kind": "EphemeralDashboard"`, 1),
			expectedPath:    "kind",
			expectedMessage: `invalid kind: "EphemeralDashboard"`,
		},
		{
			title:           "metadata project not matching the project",
			dashboard:       strings.Replace(validDashboard, `"name": "cpu"`, `"name": "cpu", "project": "other"`, 1),
			expectedPath:    "metadata.project",
			expectedMessage: "metadata.project 'other' does not match project 'myproject'",
		},
		{
			title:           "dangling panel reference",
			dashboard:       strings.Replace(validDashboard, `#/spec/panels/p1`, `#/spec/panels/p2`, 1),
			expectedPath:    "spec.layouts[0].spec.items[0].content.$ref",
			expectedMessage: `no panel found for ref "p2"`,
		},
	}

	for _, test := range testSuites {
		t.Run(test.title, func(t *testing.T) {
			perses := &fakePerses{}
			result := validation(t, connect(t, perses), test.dashboard)

			if test.expectedMessage == "" {
				if !result.Valid || len(result.Errors) > 0 || len(result.Warnings) > 0 {
					t.Errorf("expected a valid result without issue, got %+v", result)
				}
			} else {
				if result.Valid || len(result.Errors) != 1 {
					t.Fatalf("expected exactly one error, got %+v", result)
				}
				issue := result.Errors[0]
				if issue.Path != test.expectedPath || !strings.Contains(issue.Message, test.expectedMessage) {
					t.Errorf("expected error at %q containing %q, got %+v", test.expectedPath, test.expectedMessage, issue)
				}
			}
			if perses.validatedProject != test.expectedProject {
				t.Errorf("expected the dashboard to be validated in project %q, got %q", test.expectedProject, perses.validatedProject)
			}
		})
	}
}

func TestValidateDashboardWarnings(t *testing.T) {
	// p1 is displayed twice at the same place, p2 is never displayed.
	dashboard := strings.Replace(validDashboard,
		`"panels": {"p1":`, `"panels": {"p2": {"kind": "Panel", "spec": {"plugin": {"kind": "StatChart", "spec": {"calculation": "mean"}}}}, "p1":`, 1)
	dashboard = strings.Replace(dashboard,
		`"items": [`, `"items": [{"x": 0, "y": 0, "width": 30, "height": 6, "content": {"$ref": "#/spec/panels/p1"}}, `, 1)
	result := validation(t, connect(t, &fakePerses{}), dashboard)

	if result.Valid || len(result.Errors) != 1 || result.Errors[0].Path != "spec.layouts[0].spec.items[0].width" {
		t.Errorf("expected a width error, got %+v", result)
	}
	paths := make([]string, 0, len(result.Warnings))
	for _, warning := range result.Warnings {
		paths = append(paths, warning.Path)
	}
	if expected := []string{"spec.layouts[0].spec.items[1]", "spec.panels.p2"}; strings.Join(paths, ",") != strings.Join(expected, ",") {
		t.Errorf("expected warnings at %v, got %v", expected, result.Warnings)
	}
}

func TestValidateDashboardServerError(t *testing.T) {
	session := connect(t, &fakePerses{})

	dashboard := strings.Replace(validDashboard, `"mean"`, `"crash"`, 1)
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
