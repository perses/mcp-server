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

package validate

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/perses/mcp-server/pkg/tools"
	apiclient "github.com/perses/perses/pkg/client/api"
	"github.com/perses/perses/pkg/client/perseshttp"
	v1 "github.com/perses/perses/pkg/model/api/v1"
)

const (
	proxyPluginJSON  = `{"kind":"PrometheusDatasource","spec":{"proxy":{"kind":"HTTPProxy","spec":{"url":"http://prometheus:9090"}}}}`
	projectDtsJSON   = `{"kind":"Datasource","metadata":{"name":"prom","project":"test"},"spec":{"plugin":` + proxyPluginJSON + `}}`
	globalDtsJSON    = `{"kind":"GlobalDatasource","metadata":{"name":"prom"},"spec":{"plugin":` + proxyPluginJSON + `}}`
	projectVarJSON   = `{"kind":"Variable","metadata":{"name":"test","project":"test"},"spec":{"kind":"TextVariable","spec":{"name":"test","value":"test"}}}`
	globalVarJSON    = `{"kind":"GlobalVariable","metadata":{"name":"test"},"spec":{"kind":"TextVariable","spec":{"name":"test","value":"test"}}}`
	globalNotFound   = `{"message":"not found"}`
	unsavedQueryPath = "POST /proxy/unsaved/projects/test/datasources/api/v1/query"
)

type datasourceResult struct {
	Valid    bool    `json:"valid"`
	Errors   []issue `json:"errors"`
	Warnings []issue `json:"warnings"`
	Conn     *struct {
		Status    string `json:"status"`
		LatencyMS int64  `json:"latency_ms"`
		Message   string `json:"message"`
		Hint      string `json:"hint"`
	} `json:"connectivity"`
}

type issue struct {
	Path    string `json:"path"`
	Message string `json:"message"`
	Hint    string `json:"hint"`
}

func call(t *testing.T, session *mcp.ClientSession, name string, args map[string]any) (*mcp.CallToolResult, string) {
	t.Helper()
	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
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

func decode(t *testing.T, result *mcp.CallToolResult, text string) datasourceResult {
	t.Helper()
	if result.IsError {
		t.Fatalf("unexpected tool error: %s", text)
	}
	var decoded datasourceResult
	if err := json.Unmarshal([]byte(text), &decoded); err != nil {
		t.Fatalf("decoding %s: %v", text, err)
	}
	return decoded
}

func rawJSON(document string) map[string]any {
	var value map[string]any
	if err := json.Unmarshal([]byte(document), &value); err != nil {
		panic(err)
	}
	return value
}

func findRequest(requests []capturedRequest, method, path string) *capturedRequest {
	for i := range requests {
		if requests[i].method == method && requests[i].path == path {
			return &requests[i]
		}
	}
	return nil
}

func TestValidationRequests(t *testing.T) {
	testCases := []struct {
		name     string
		tool     string
		args     map[string]any
		path     string
		expected string
		entity   any
	}{
		{
			name: "dashboard", tool: "perses_validate_dashboard", args: map[string]any{"dashboard": dashboardJSON},
			path: "/api/validate/dashboards", expected: dashboardJSON, entity: &v1.Dashboard{},
		},
		{
			name: "project datasource from kind", tool: "perses_validate_datasource", args: map[string]any{"datasource": rawJSON(projectDtsJSON)},
			path: "/api/validate/datasources", expected: projectDtsJSON, entity: &v1.Datasource{},
		},
		{
			name: "project datasource from project, as string", tool: "perses_validate_datasource",
			args: map[string]any{"project": "test", "datasource": `{"metadata":{"name":"prom"},"spec":{"plugin":` + proxyPluginJSON + `}}`},
			path: "/api/validate/datasources", expected: projectDtsJSON, entity: &v1.Datasource{},
		},
		{
			name: "global datasource from kind", tool: "perses_validate_datasource", args: map[string]any{"datasource": rawJSON(globalDtsJSON)},
			path: "/api/validate/globaldatasources", expected: globalDtsJSON, entity: &v1.GlobalDatasource{},
		},
		{
			name: "global datasource from scope", tool: "perses_validate_datasource",
			args: map[string]any{"scope": "global", "datasource": rawJSON(`{"metadata":{"name":"prom"},"spec":{"plugin":` + proxyPluginJSON + `}}`)},
			path: "/api/validate/globaldatasources", expected: globalDtsJSON, entity: &v1.GlobalDatasource{},
		},
		{
			name: "project variable", tool: "perses_validate_variable", args: map[string]any{"variable": rawJSON(projectVarJSON)},
			path: "/api/validate/variables", expected: projectVarJSON, entity: &v1.Variable{},
		},
		{
			name: "global variable from scope", tool: "perses_validate_variable",
			args: map[string]any{"scope": "global", "variable": `{"metadata":{"name":"test"},"spec":{"kind":"TextVariable","spec":{"name":"test","value":"test"}}}`},
			path: "/api/validate/globalvariables", expected: globalVarJSON, entity: &v1.GlobalVariable{},
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			if err := json.Unmarshal([]byte(testCase.expected), testCase.entity); err != nil {
				t.Fatalf("invalid test fixture: %v", err)
			}
			expectedBody, err := json.Marshal(testCase.entity)
			if err != nil {
				t.Fatal(err)
			}
			perses := &fakePerses{status: http.StatusOK, routes: map[string]fakeResponse{
				"GET /api/v1/globaldatasources/prom": {status: http.StatusNotFound, body: globalNotFound},
			}}
			result, text := call(t, connect(t, perses), testCase.tool, testCase.args)
			if result.IsError || text != `{"valid":true}` {
				t.Fatalf("unexpected validation result: IsError=%v, %s", result.IsError, text)
			}
			request := findRequest(perses.received(), http.MethodPost, testCase.path)
			if request == nil {
				t.Fatalf("no request to %s: %#v", testCase.path, perses.received())
			}
			if request.body != string(expectedBody) {
				t.Errorf("unexpected body %s; expected %s", request.body, expectedBody)
			}
			if request.contentType != "application/json" || request.authorization != testAuthorization {
				t.Errorf("unexpected request headers: %#v", request)
			}
		})
	}
}

func TestValidationServerErrors(t *testing.T) {
	testCases := []struct {
		name    string
		tool    string
		args    map[string]any
		path    string
		status  int
		body    string
		errPath string
		isError bool
	}{
		{
			name: "datasource plugin field", tool: "perses_validate_datasource", args: map[string]any{"datasource": rawJSON(projectDtsJSON)},
			path: "POST /api/validate/datasources", status: http.StatusBadRequest,
			body:    `{"message":"invalid datasource prom: spec.foo: field not allowed:\n    /cue/prometheus.cue:12:3"}`,
			errPath: "spec.plugin.spec.foo",
		},
		{
			name: "datasource unknown plugin", tool: "perses_validate_datasource", args: map[string]any{"datasource": rawJSON(globalDtsJSON)},
			path: "POST /api/validate/globaldatasources", status: http.StatusBadRequest,
			body:    `{"message":"schema not found for plugin PromDatasource with version v0"}`,
			errPath: "spec.plugin.kind",
		},
		{
			name: "variable plugin field", tool: "perses_validate_variable", args: map[string]any{"variable": rawJSON(projectVarJSON)},
			path: "POST /api/validate/variables", status: http.StatusBadRequest,
			body:    `{"message":"invalid variable : spec.foo: field not allowed"}`,
			errPath: "spec.spec.plugin.spec.foo",
		},
		{
			name: "server failure", tool: "perses_validate_datasource", args: map[string]any{"datasource": rawJSON(projectDtsJSON)},
			path: "POST /api/validate/datasources", status: http.StatusInternalServerError, body: `{"message":"boom"}`, isError: true,
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			perses := &fakePerses{status: http.StatusNotFound, body: globalNotFound, routes: map[string]fakeResponse{
				testCase.path: {status: testCase.status, body: testCase.body},
			}}
			result, text := call(t, connect(t, perses), testCase.tool, testCase.args)
			if testCase.isError {
				if !result.IsError {
					t.Fatalf("expected a tool error, got %s", text)
				}
				return
			}
			decoded := decode(t, result, text)
			if decoded.Valid || len(decoded.Errors) != 1 || decoded.Errors[0].Path != testCase.errPath || decoded.Errors[0].Hint == "" {
				t.Errorf("unexpected validation result: %s", text)
			}
		})
	}
}

func TestInputErrors(t *testing.T) {
	testCases := []struct {
		name    string
		args    map[string]any
		isError bool
		errPath string
	}{
		{name: "neither datasource nor name", args: map[string]any{"project": "test"}, isError: true},
		{name: "both datasource and name", args: map[string]any{"name": "prom", "datasource": rawJSON(projectDtsJSON)}, isError: true},
		{name: "invalid scope", args: map[string]any{"scope": "dashboard", "datasource": rawJSON(projectDtsJSON)}, isError: true},
		{name: "kind and scope mismatch", args: map[string]any{"scope": "global", "datasource": rawJSON(projectDtsJSON)}, errPath: "kind"},
		{name: "invalid kind", args: map[string]any{"datasource": rawJSON(`{"kind":"Dashboard","metadata":{"name":"x"}}`)}, errPath: "kind"},
		{name: "unknown scope", args: map[string]any{"datasource": rawJSON(`{"metadata":{"name":"x"},"spec":{}}`)}, errPath: ""},
		{name: "missing name", args: map[string]any{"project": "test", "datasource": rawJSON(`{"spec":{}}`)}, errPath: "metadata.name"},
		{name: "project mismatch", args: map[string]any{"project": "other", "datasource": rawJSON(projectDtsJSON)}, errPath: "metadata.project"},
		{name: "malformed string", args: map[string]any{"datasource": `{"kind":`}, errPath: "datasource"},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			perses := &fakePerses{status: http.StatusOK}
			result, text := call(t, connect(t, perses), "perses_validate_datasource", testCase.args)
			if testCase.isError {
				if !result.IsError {
					t.Fatalf("expected a tool error, got %s", text)
				}
			} else {
				decoded := decode(t, result, text)
				if decoded.Valid || len(decoded.Errors) == 0 || decoded.Errors[0].Path != testCase.errPath {
					t.Errorf("unexpected validation result: %s", text)
				}
			}
			if requests := perses.received(); len(requests) != 0 {
				t.Errorf("expected no request, got %#v", requests)
			}
		})
	}
}

func TestDatasourceWarnings(t *testing.T) {
	withPlugin := func(plugin string) map[string]any {
		return rawJSON(`{"kind":"Datasource","metadata":{"name":"prom","project":"test"},"spec":{"plugin":` + plugin + `}}`)
	}
	testCases := []struct {
		name       string
		datasource map[string]any
		routes     map[string]fakeResponse
		path       string
		isError    bool
	}{
		{
			name:       "direct URL only",
			datasource: withPlugin(`{"kind":"PrometheusDatasource","spec":{"directUrl":"http://cortex:9009"}}`),
			path:       "spec.plugin.spec.directUrl",
		},
		{
			name:       "localhost proxy",
			datasource: withPlugin(`{"kind":"PrometheusDatasource","spec":{"proxy":{"kind":"HTTPProxy","spec":{"url":"http://127.0.0.1:9090"}}}}`),
			path:       "spec.plugin.spec.proxy.spec.url",
		},
		{
			name:       "api path in URL",
			datasource: withPlugin(`{"kind":"PrometheusDatasource","spec":{"proxy":{"kind":"HTTPProxy","spec":{"url":"http://prometheus:9090/api/v1/"}}}}`),
			path:       "spec.plugin.spec.proxy.spec.url",
		},
		{
			name: "blocked endpoints",
			datasource: withPlugin(`{"kind":"PrometheusDatasource","spec":{"proxy":{"kind":"HTTPProxy","spec":{"url":"http://prometheus:9090",` +
				`"allowedEndpoints":[{"endpointPattern":"/api/v1/query$","method":"POST"}]}}}}`),
			path: "spec.plugin.spec.proxy.spec.allowedEndpoints",
		},
		{
			name:       "hides a global datasource",
			datasource: rawJSON(projectDtsJSON),
			routes:     map[string]fakeResponse{"GET /api/v1/globaldatasources/prom": {status: http.StatusOK, body: globalDtsJSON}},
			path:       "metadata.name",
		},
		{
			name:       "second default",
			datasource: rawJSON(`{"kind":"Datasource","metadata":{"name":"prom","project":"test"},"spec":{"default":true,"plugin":` + proxyPluginJSON + `}}`),
			routes: map[string]fakeResponse{"GET /api/v1/projects/test/datasources": {status: http.StatusOK,
				body: `[{"kind":"Datasource","metadata":{"name":"other","project":"test"},"spec":{"default":true,"plugin":` + proxyPluginJSON + `}}]`}},
			path:    "spec.default",
			isError: true,
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			routes := map[string]fakeResponse{"GET /api/v1/globaldatasources/prom": {status: http.StatusNotFound, body: globalNotFound}}
			for route, response := range testCase.routes {
				routes[route] = response
			}
			perses := &fakePerses{status: http.StatusOK, routes: routes}
			result, text := call(t, connect(t, perses), "perses_validate_datasource", map[string]any{"datasource": testCase.datasource})
			decoded := decode(t, result, text)
			issues := decoded.Warnings
			if testCase.isError {
				issues = decoded.Errors
				if decoded.Valid {
					t.Errorf("expected invalid: %s", text)
				}
			} else if !decoded.Valid {
				t.Errorf("expected valid: %s", text)
			}
			if len(issues) != 1 || issues[0].Path != testCase.path || issues[0].Hint == "" {
				t.Errorf("unexpected issues: %s", text)
			}
		})
	}
}

func TestConnectivity(t *testing.T) {
	testCases := []struct {
		name     string
		response fakeResponse
		status   string
	}{
		{name: "ok", response: fakeResponse{status: http.StatusOK, body: `{"status":"success"}`}, status: statusOK},
		{name: "unknown host", response: fakeResponse{status: http.StatusInternalServerError,
			body: `{"message":"dial tcp: lookup thanos-global-query on 10.0.0.10:53: no such host"}`}, status: statusDNS},
		{name: "refused", response: fakeResponse{status: http.StatusBadGateway,
			body: `{"message":"dial tcp 10.0.0.1:9090: connect: connection refused"}`}, status: statusUnreachable},
		{name: "upstream unauthorized", response: fakeResponse{status: http.StatusUnauthorized, body: "Unauthorized"}, status: statusAuth},
		{name: "allowed endpoints", response: fakeResponse{status: http.StatusForbidden,
			body: `{"message":"you are not allowed to use this endpoint \"/api/v1/query\" with the HTTP method GET"}`}, status: statusBlocked},
		{name: "missing permission", response: fakeResponse{status: http.StatusForbidden,
			body: `{"message":"missing 'create' permission in 'test' project for 'Datasource' kind"}`}, status: statusForbidden},
		{name: "timeout", response: fakeResponse{status: http.StatusBadGateway,
			body: `{"message":"dial tcp 10.0.0.1:9090: i/o timeout"}`}, status: statusTimeout},
		{name: "wrong path", response: fakeResponse{status: http.StatusNotFound, body: "404 page not found"}, status: statusError},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			perses := &fakePerses{status: http.StatusOK, routes: map[string]fakeResponse{
				"GET /api/v1/globaldatasources/prom": {status: http.StatusNotFound, body: globalNotFound},
				unsavedQueryPath:                     testCase.response,
			}}
			result, text := call(t, connect(t, perses), "perses_validate_datasource",
				map[string]any{"datasource": rawJSON(projectDtsJSON), "check_connectivity": true})
			decoded := decode(t, result, text)
			if decoded.Conn == nil || decoded.Conn.Status != testCase.status {
				t.Fatalf("expected status %s: %s", testCase.status, text)
			}
			if testCase.status != statusOK && (decoded.Conn.Message == "" || decoded.Conn.Hint == "") {
				t.Errorf("expected message and hint: %s", text)
			}
			request := findRequest(perses.received(), http.MethodPost, "/proxy/unsaved/projects/test/datasources/api/v1/query")
			if request == nil {
				t.Fatal("no request to the unsaved proxy")
			}
			if request.query != "query=vector%281%29" {
				t.Errorf("unexpected query %q", request.query)
			}
			var body struct {
				Method string         `json:"method"`
				Spec   map[string]any `json:"spec"`
			}
			if err := json.Unmarshal([]byte(request.body), &body); err != nil || body.Method != http.MethodGet || body.Spec["plugin"] == nil {
				t.Errorf("unexpected unsaved proxy body %s (%v)", request.body, err)
			}
		})
	}
}

func TestConnectivityOfSavedDatasource(t *testing.T) {
	perses := &fakePerses{status: http.StatusNotFound, body: globalNotFound, routes: map[string]fakeResponse{
		"GET /api/v1/projects/test/datasources/prom":             {status: http.StatusOK, body: projectDtsJSON},
		"GET /proxy/projects/test/datasources/prom/api/v1/query": {status: http.StatusOK, body: `{"status":"success"}`},
	}}
	result, text := call(t, connect(t, perses), "perses_validate_datasource",
		map[string]any{"project": "test", "name": "prom", "check_connectivity": true})
	decoded := decode(t, result, text)
	if !decoded.Valid || decoded.Conn == nil || decoded.Conn.Status != statusOK {
		t.Fatalf("unexpected result: %s", text)
	}
	if findRequest(perses.received(), http.MethodPost, "/api/validate/datasources") != nil {
		t.Error("a saved datasource must not be sent to the validation API")
	}

	result, text = call(t, connect(t, perses), "perses_validate_datasource", map[string]any{"project": "test", "name": "missing"})
	decoded = decode(t, result, text)
	if decoded.Valid || len(decoded.Errors) != 1 || decoded.Errors[0].Path != "name" {
		t.Errorf("unexpected result for a missing datasource: %s", text)
	}
}

func TestConnectivityWithoutRequest(t *testing.T) {
	testCases := []struct {
		name   string
		plugin string
		status string
	}{
		{name: "direct URL only", plugin: `{"kind":"PrometheusDatasource","spec":{"directUrl":"http://cortex:9009"}}`, status: statusNoProxy},
		{name: "every probe blocked", plugin: `{"kind":"PrometheusDatasource","spec":{"proxy":{"kind":"HTTPProxy","spec":{"url":"http://prometheus:9090",` +
			`"allowedEndpoints":[{"endpointPattern":"/api/v1/query_range","method":"POST"}]}}}}`, status: statusSkipped},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			perses := &fakePerses{status: http.StatusNotFound, body: globalNotFound, routes: map[string]fakeResponse{
				"POST /api/validate/datasources": {status: http.StatusOK},
			}}
			result, text := call(t, connect(t, perses), "perses_validate_datasource", map[string]any{"check_connectivity": true,
				"datasource": rawJSON(`{"kind":"Datasource","metadata":{"name":"prom","project":"test"},"spec":{"plugin":` + testCase.plugin + `}}`)})
			decoded := decode(t, result, text)
			if decoded.Conn == nil || decoded.Conn.Status != testCase.status {
				t.Fatalf("expected status %s: %s", testCase.status, text)
			}
			for _, request := range perses.received() {
				if strings.HasPrefix(request.path, "/proxy/") {
					t.Errorf("unexpected proxy request %s", request.path)
				}
			}
		})
	}
}

func TestValidationToolsMetadata(t *testing.T) {
	expected := []string{"perses_validate_dashboard", "perses_validate_datasource", "perses_validate_variable"}
	toolset := New(apiclient.NewWithClient(&perseshttp.RESTClient{})).GetTools()
	if len(toolset) != len(expected) {
		t.Fatalf("expected %d tools, got %d", len(expected), len(toolset))
	}
	for i, tool := range toolset {
		name := tool.MCPTool.Name
		if name != expected[i] {
			t.Errorf("unexpected tool %s, expected %s", name, expected[i])
		}
		annotations := tool.MCPTool.Annotations
		if tool.IsWriteTool || !annotations.ReadOnlyHint || !annotations.IdempotentHint {
			t.Errorf("%s: expected read-only, idempotent metadata", name)
		}
		if annotations.DestructiveHint == nil || *annotations.DestructiveHint ||
			annotations.OpenWorldHint == nil || *annotations.OpenWorldHint {
			t.Errorf("%s: expected non-destructive, closed-world metadata", name)
		}
		if tool.ResourceType != tools.ValidateResource {
			t.Errorf("%s: unexpected resource %q", name, tool.ResourceType)
		}
		if !strings.Contains(tool.MCPTool.Description, "without saving") {
			t.Errorf("%s: description must say that nothing is saved", name)
		}
	}
}
