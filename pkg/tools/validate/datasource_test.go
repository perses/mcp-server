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
	"reflect"
	"strconv"
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
	unsavedQueryPath = "POST /proxy/unsaved/projects/test/datasources/api/v1/status/buildinfo"
)

type datasourceResult struct {
	Valid    bool     `json:"valid"`
	Error    string   `json:"error"`
	Warnings []string `json:"warnings"`
	Conn     *struct {
		Status     string `json:"status"`
		HTTPStatus int    `json:"http_status"`
		LatencyMS  int64  `json:"latency_ms"`
		Message    string `json:"message"`
		Hint       string `json:"hint"`
	} `json:"connectivity"`
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
			name: "dashboard", tool: "perses_validate_dashboard", args: map[string]any{"dashboard": rawJSON(dashboardJSON)},
			path: "/api/validate/dashboards", expected: dashboardJSON, entity: &v1.Dashboard{},
		},
		{
			name: "project datasource", tool: "perses_validate_datasource", args: map[string]any{"datasource": rawJSON(projectDtsJSON)},
			path: "/api/validate/datasources", expected: projectDtsJSON, entity: &v1.Datasource{},
		},
		{
			name: "global datasource", tool: "perses_validate_datasource", args: map[string]any{"datasource": rawJSON(globalDtsJSON)},
			path: "/api/validate/globaldatasources", expected: globalDtsJSON, entity: &v1.GlobalDatasource{},
		},
		{
			name: "project variable", tool: "perses_validate_variable", args: map[string]any{"variable": rawJSON(projectVarJSON)},
			path: "/api/validate/variables", expected: projectVarJSON, entity: &v1.Variable{},
		},
		{
			name: "global variable", tool: "perses_validate_variable", args: map[string]any{"variable": rawJSON(globalVarJSON)},
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
		message string
	}{
		{
			name: "datasource plugin field", tool: "perses_validate_datasource", args: map[string]any{"datasource": rawJSON(projectDtsJSON)},
			path: "POST /api/validate/datasources", status: http.StatusBadRequest,
			body:    `{"message":"invalid datasource prom: spec.foo: field not allowed:\n    /cue/prometheus.cue:12:3"}`,
			message: "invalid datasource prom: spec.foo: field not allowed:\n    /cue/prometheus.cue:12:3",
		},
		{
			name: "datasource unknown plugin", tool: "perses_validate_datasource", args: map[string]any{"datasource": rawJSON(globalDtsJSON)},
			path: "POST /api/validate/globaldatasources", status: http.StatusBadRequest,
			body:    `{"message":"schema not found for plugin PromDatasource with version v0"}`,
			message: "schema not found for plugin PromDatasource with version v0",
		},
		{
			name: "variable plugin field", tool: "perses_validate_variable", args: map[string]any{"variable": rawJSON(projectVarJSON)},
			path: "POST /api/validate/variables", status: http.StatusBadRequest,
			body:    `{"message":"invalid variable : spec.foo: field not allowed"}`,
			message: "invalid variable : spec.foo: field not allowed",
		},
		{
			name: "server failure", tool: "perses_validate_datasource", args: map[string]any{"datasource": rawJSON(projectDtsJSON)},
			path: "POST /api/validate/datasources", status: http.StatusInternalServerError, body: `{"message":"boom"}`,
			message: "internal server error",
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			perses := &fakePerses{status: http.StatusNotFound, body: globalNotFound, routes: map[string]fakeResponse{
				testCase.path: {status: testCase.status, body: testCase.body},
			}}
			result, text := call(t, connect(t, perses), testCase.tool, testCase.args)
			if !result.IsError || !strings.Contains(text, testCase.message) {
				t.Errorf("expected the error of the Perses client unchanged, got %s", text)
			}
		})
	}
}

func TestInputErrors(t *testing.T) {
	testCases := []struct {
		name    string
		args    map[string]any
		isError bool
		// message is contained in the error of the invalid result.
		message string
	}{
		{name: "neither datasource nor name", args: map[string]any{"project": "test"}, isError: true},
		{name: "both datasource and name", args: map[string]any{"name": "prom", "datasource": rawJSON(projectDtsJSON)}, isError: true},
		{name: "missing kind", args: map[string]any{"project": "test", "datasource": rawJSON(`{"metadata":{"name":"x"},"spec":{}}`)},
			message: `invalid kind: "" for a Datasource type`},
		{name: "invalid kind", args: map[string]any{"project": "test", "datasource": rawJSON(`{"kind":"Dashboard","metadata":{"name":"x"}}`)},
			message: `invalid kind: "Dashboard" for a Datasource type`},
		{name: "empty spec", args: map[string]any{"project": "test", "datasource": rawJSON(`{"kind":"Datasource","spec":{}}`)}, message: "spec cannot be empty"},
		{name: "non-string project", args: map[string]any{"project": "test",
			"datasource": rawJSON(`{"kind":"Datasource","metadata":{"name":"prom","project":42},"spec":{}}`)}, message: "cannot unmarshal number"},
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
				if decoded.Valid || !strings.Contains(decoded.Error, testCase.message) {
					t.Errorf("expected an error containing %q, got %s", testCase.message, text)
				}
			}
			if requests := perses.received(); len(requests) != 0 {
				t.Errorf("expected no request, got %#v", requests)
			}
		})
	}
}

// TestOtherDatasourceWarnings checks the conflicts with the saved datasources that the validation API does not report.
func TestOtherDatasourceWarnings(t *testing.T) {
	testCases := []struct {
		name       string
		datasource map[string]any
		routes     map[string]fakeResponse
		warning    string
	}{
		{
			name:       "hides a global datasource",
			datasource: rawJSON(projectDtsJSON),
			routes:     map[string]fakeResponse{"GET /api/v1/globaldatasources/prom": {status: http.StatusOK, body: globalDtsJSON}},
			warning:    "hides the global datasource",
		},
		{
			name:       "second default",
			datasource: rawJSON(`{"kind":"Datasource","metadata":{"name":"prom","project":"test"},"spec":{"default":true,"plugin":` + proxyPluginJSON + `}}`),
			routes: map[string]fakeResponse{"GET /api/v1/projects/test/datasources": {status: http.StatusOK,
				body: `[{"kind":"Datasource","metadata":{"name":"other","project":"test"},"spec":{"default":true,"plugin":` + proxyPluginJSON + `}}]`}},
			warning: "is already the default PrometheusDatasource",
		},
		{
			name:       "second default with another case on a case-sensitive server",
			datasource: rawJSON(`{"kind":"Datasource","metadata":{"name":"Prom","project":"test"},"spec":{"default":true,"plugin":` + proxyPluginJSON + `}}`),
			routes: map[string]fakeResponse{
				"GET /api/config": {status: http.StatusOK, body: `{"database":{"file":{"folder":"/perses","case_sensitive":true}}}`},
				"GET /api/v1/projects/test/datasources": {status: http.StatusOK,
					body: `[{"kind":"Datasource","metadata":{"name":"prom","project":"test"},"spec":{"default":true,"plugin":` + proxyPluginJSON + `}}]`},
			},
			warning: "is already the default PrometheusDatasource",
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
			if !decoded.Valid {
				t.Errorf("expected valid: %s", text)
			}
			if len(decoded.Warnings) != 1 || !strings.Contains(decoded.Warnings[0], testCase.warning) {
				t.Errorf("expected one warning containing %q, got %s", testCase.warning, text)
			}
		})
	}
}

func TestConnectivity(t *testing.T) {
	testCases := []struct {
		name       string
		response   fakeResponse
		status     string
		httpStatus int
		message    string
	}{
		{name: "ok", response: fakeResponse{status: http.StatusOK, body: `{"status":"success"}`}, status: statusOK},
		{name: "Perses error", response: fakeResponse{status: http.StatusBadGateway,
			body: `{"message":"dial tcp 10.0.0.1:9090: connect: connection refused"}`},
			status: statusError, httpStatus: http.StatusBadGateway, message: "dial tcp 10.0.0.1:9090: connect: connection refused"},
		{name: "datasource error", response: fakeResponse{status: http.StatusUnauthorized, body: " Unauthorized\n"},
			status: statusError, httpStatus: http.StatusUnauthorized, message: "Unauthorized"},
		{name: "error field", response: fakeResponse{status: http.StatusBadRequest, body: `{"status":"error","error":"bad query"}`},
			status: statusError, httpStatus: http.StatusBadRequest, message: "bad query"},
		{name: "error body larger than the proxy limit", response: fakeResponse{status: http.StatusInternalServerError,
			body: strings.Repeat("x", 1<<20+1024)}, status: statusError, httpStatus: http.StatusInternalServerError,
			message: strings.Repeat("x", maxMessageLength) + "…"},
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
			if decoded.Conn.HTTPStatus != testCase.httpStatus || decoded.Conn.Message != testCase.message {
				t.Errorf("expected http_status %d and message %q: %s", testCase.httpStatus, testCase.message, text)
			}
			request := findRequest(perses.received(), http.MethodPost, "/proxy/unsaved/projects/test/datasources/api/v1/status/buildinfo")
			if request == nil {
				t.Fatal("no request to the unsaved proxy")
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
		"GET /api/v1/projects/test/datasources/prom":                        {status: http.StatusOK, body: projectDtsJSON},
		"GET /proxy/projects/test/datasources/prom/api/v1/status/buildinfo": {status: http.StatusOK, body: `{"status":"success"}`},
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
	if !result.IsError || !strings.Contains(text, "not found") {
		t.Errorf("expected a tool error for a missing datasource: %s", text)
	}
}

// TestDefaultOfSameDatasourceWithAnotherCase checks that a datasource is not reported as a second default of
// itself when the server, case-insensitive, stores its name in lower case.
func TestDefaultOfSameDatasourceWithAnotherCase(t *testing.T) {
	saved := `{"kind":"Datasource","metadata":{"name":"prom","project":"test"},"spec":{"default":true,"plugin":` + proxyPluginJSON + `}}`
	for _, config := range []fakeResponse{
		{status: http.StatusOK, body: `{"database":{"file":{"folder":"/perses","case_sensitive":false}}}`},
		{status: http.StatusInternalServerError, body: `{"message":"unavailable"}`},
	} {
		perses := &fakePerses{status: http.StatusOK, routes: map[string]fakeResponse{
			"GET /api/config":                            config,
			"GET /api/v1/globaldatasources/Prom":         {status: http.StatusNotFound, body: globalNotFound},
			"GET /api/v1/globaldatasources/prom":         {status: http.StatusNotFound, body: globalNotFound},
			"GET /api/v1/projects/test/datasources":      {status: http.StatusOK, body: "[" + saved + "]"},
			"GET /api/v1/projects/test/datasources/Prom": {status: http.StatusOK, body: saved},
		}}
		result, text := call(t, connect(t, perses), "perses_validate_datasource", map[string]any{"datasource": rawJSON(
			`{"kind":"Datasource","metadata":{"name":"Prom","project":"test"},"spec":{"default":true,"plugin":` + proxyPluginJSON + `}}`)})
		if decoded := decode(t, result, text); !decoded.Valid {
			t.Errorf("expected valid: %s", text)
		}
		result, text = call(t, connect(t, perses), "perses_validate_datasource", map[string]any{"project": "test", "name": "Prom"})
		if decoded := decode(t, result, text); !decoded.Valid {
			t.Errorf("expected valid for the saved datasource: %s", text)
		}
	}
}

// TestConnectivityWithAllowedEndpoints checks that, like the Perses UI, the health endpoint is added to the
// allowedEndpoints of an unsaved datasource, and that a saved datasource blocking it reports the error of Perses.
func TestConnectivityWithAllowedEndpoints(t *testing.T) {
	const blockedMessage = `you are not allowed to use this endpoint "/api/v1/status/buildinfo" with the HTTP method GET`
	restricted := `{"kind":"Datasource","metadata":{"name":"prom","project":"test"},"spec":{"plugin":{"kind":"PrometheusDatasource",` +
		`"spec":{"proxy":{"kind":"HTTPProxy","spec":{"url":"http://prometheus:9090",` +
		`"allowedEndpoints":[{"endpointPattern":"/api/v1/query","method":"POST"}]}}}}}}`
	perses := &fakePerses{status: http.StatusNotFound, body: globalNotFound, routes: map[string]fakeResponse{
		"POST /api/validate/datasources":             {status: http.StatusOK},
		"GET /api/v1/projects/test/datasources/prom": {status: http.StatusOK, body: restricted},
		unsavedQueryPath:                             {status: http.StatusOK, body: `{"status":"success"}`},
		"GET /proxy/projects/test/datasources/prom/api/v1/status/buildinfo": {status: http.StatusForbidden,
			body: `{"message":` + strconv.Quote(blockedMessage) + `}`},
	}}
	unsavedEndpoints := func(document string) []any {
		t.Helper()
		result, text := call(t, connect(t, perses), "perses_validate_datasource",
			map[string]any{"datasource": rawJSON(document), "check_connectivity": true})
		if decoded := decode(t, result, text); decoded.Conn == nil || decoded.Conn.Status != statusOK {
			t.Fatalf("unexpected result for the unsaved datasource: %s", text)
		}
		requests := perses.received()
		request := requests[len(requests)-1]
		var body struct {
			Method string `json:"method"`
			Spec   struct {
				Plugin struct {
					Spec struct {
						Proxy struct {
							Spec struct {
								AllowedEndpoints []any `json:"allowedEndpoints"`
							} `json:"spec"`
						} `json:"proxy"`
					} `json:"spec"`
				} `json:"plugin"`
			} `json:"spec"`
		}
		if request.path != "/proxy/unsaved/projects/test/datasources/api/v1/status/buildinfo" ||
			json.Unmarshal([]byte(request.body), &body) != nil || body.Method != http.MethodGet {
			t.Fatalf("unexpected unsaved proxy request %s %s", request.path, request.body)
		}
		return body.Spec.Plugin.Spec.Proxy.Spec.AllowedEndpoints
	}

	endpoints := unsavedEndpoints(restricted)
	expected := []any{
		map[string]any{"endpointPattern": "/api/v1/query", "method": "POST"},
		map[string]any{"endpointPattern": "/api/v1/status/buildinfo", "method": "GET"},
	}
	if !reflect.DeepEqual(endpoints, expected) {
		t.Errorf("unexpected allowedEndpoints %v", endpoints)
	}
	if endpoints := unsavedEndpoints(projectDtsJSON); len(endpoints) != 0 {
		t.Errorf("allowedEndpoints must stay empty: %v", endpoints)
	}

	result, text := call(t, connect(t, perses), "perses_validate_datasource",
		map[string]any{"project": "test", "name": "prom", "check_connectivity": true})
	decoded := decode(t, result, text)
	if decoded.Conn == nil || decoded.Conn.Status != statusError || decoded.Conn.HTTPStatus != http.StatusForbidden ||
		decoded.Conn.Message != blockedMessage {
		t.Fatalf("expected the error of the Perses proxy for the saved datasource: %s", text)
	}
}

// TestConnectivityWithDisabledProxy checks that the datasources whose proxy is disabled by the Perses configuration are
// not queried.
func TestConnectivityWithDisabledProxy(t *testing.T) {
	testCases := []struct {
		name    string
		config  string
		dts     string
		skipped bool
	}{
		{name: "project proxy disabled", config: `{"datasource":{"project":{"disable":true}}}`,
			dts: `{"kind":"Datasource","metadata":{"name":"prom","project":"test"},"spec":{"plugin":` + proxyPluginJSON + `}}`, skipped: true},
		{name: "global proxy disabled", config: `{"datasource":{"global":{"disable":true}}}`,
			dts: `{"kind":"GlobalDatasource","metadata":{"name":"prom"},"spec":{"plugin":` + proxyPluginJSON + `}}`, skipped: true},
		{name: "other scope disabled", config: `{"datasource":{"global":{"disable":true}}}`,
			dts: `{"kind":"Datasource","metadata":{"name":"prom","project":"test"},"spec":{"plugin":` + proxyPluginJSON + `}}`},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			perses := &fakePerses{status: http.StatusNotFound, body: `{"message":"document not found"}`, routes: map[string]fakeResponse{
				"GET /api/config":                      {status: http.StatusOK, body: testCase.config},
				"POST /api/validate/datasources":       {status: http.StatusOK},
				"POST /api/validate/globaldatasources": {status: http.StatusOK},
				unsavedQueryPath:                       {status: http.StatusOK},
			}}
			result, text := call(t, connect(t, perses), "perses_validate_datasource",
				map[string]any{"datasource": rawJSON(testCase.dts), "check_connectivity": true})
			decoded := decode(t, result, text)
			if !testCase.skipped {
				if decoded.Conn == nil || decoded.Conn.Status != statusOK {
					t.Fatalf("expected status ok: %s", text)
				}
				return
			}
			if decoded.Conn == nil || decoded.Conn.Status != statusSkipped || !strings.Contains(decoded.Conn.Message, ".disable is set") {
				t.Fatalf("expected the check to be skipped: %s", text)
			}
			for _, request := range perses.received() {
				if strings.HasPrefix(request.path, "/proxy/") {
					t.Errorf("unexpected proxy request %s", request.path)
				}
			}
		})
	}
}

func TestConnectivityWithoutRequest(t *testing.T) {
	testCases := []struct {
		name   string
		plugin string
		status string
	}{
		{name: "direct URL only", plugin: `{"kind":"PrometheusDatasource","spec":{"directUrl":"http://cortex:9009"}}`, status: statusSkipped},
		{name: "SQL proxy", plugin: `{"kind":"PostgresDatasource","spec":{"proxy":{"kind":"SQLProxy","spec":{"driver":"postgres","host":"db:5432"}}}}`,
			status: statusSkipped},
		{name: "plugin without health endpoint", plugin: `{"kind":"LokiDatasource","spec":{"proxy":{"kind":"HTTPProxy","spec":{"url":"http://loki:3100"}}}}`,
			status: statusSkipped},
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
