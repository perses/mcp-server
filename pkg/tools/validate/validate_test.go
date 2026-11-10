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
	"github.com/perses/mcp-server/pkg/tools/internal/tooltest"
	apiclient "github.com/perses/perses/pkg/client/api"
	"github.com/perses/perses/pkg/client/perseshttp"
	v1 "github.com/perses/perses/pkg/model/api/v1"
)

const (
	dashboardJSON  = `{"kind":"Dashboard","metadata":{"name":"test","project":"test"},"spec":{"duration":"1h","panels":{},"layouts":[]}}`
	projectVarJSON = `{"kind":"Variable","metadata":{"name":"test","project":"test"},"spec":{"kind":"TextVariable","spec":{"name":"test","value":"test"}}}`
	globalVarJSON  = `{"kind":"GlobalVariable","metadata":{"name":"test"},"spec":{"kind":"TextVariable","spec":{"name":"test","value":"test"}}}`
)

type fakeResponse struct {
	status int
	body   string
}

// fakePerses answers the requests with the response of their route ("METHOD /path"), or else with status and an
// empty body.
func fakePerses(status int, routes map[string]fakeResponse) *tooltest.Perses {
	return &tooltest.Perses{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response, ok := routes[r.Method+" "+r.URL.Path]
		if !ok {
			response = fakeResponse{status: status}
		}
		w.WriteHeader(response.status)
		_, _ = w.Write([]byte(response.body))
	})}
}

func connect(t *testing.T, perses http.Handler) *mcp.ClientSession {
	t.Helper()
	return tooltest.Connect(t, New(tooltest.NewClient(t, perses)).GetTools())
}

func TestRejectsInvalidPayloadWithoutRequest(t *testing.T) {
	testCases := []struct {
		name     string
		tool     string
		args     map[string]any
		expected string
	}{
		{
			name: "dashboard with another kind", tool: "perses_validate_dashboard",
			args:     map[string]any{"dashboard": rawJSON(`{"kind":"Datasource","metadata":{"name":"test","project":"test"},"spec":{"duration":"1h"}}`)},
			expected: `{"valid":false,"error":"invalid kind: \"Datasource\" for a Dashboard type"}`,
		},
		{
			name: "malformed project variable", tool: "perses_validate_variable",
			args:     map[string]any{"variable": rawJSON(strings.Replace(projectVarJSON, `"TextVariable"`, `"NotAVariable"`, 1))},
			expected: `{"valid":false,"error":"unknown variable.kind \"NotAVariable\" used"}`,
		},
		{
			name: "malformed global variable", tool: "perses_validate_variable",
			args:     map[string]any{"variable": rawJSON(strings.Replace(globalVarJSON, `"TextVariable"`, `"NotAVariable"`, 1))},
			expected: `{"valid":false,"error":"unknown variable.kind \"NotAVariable\" used"}`,
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			perses := fakePerses(http.StatusOK, nil)
			result, text := tooltest.CallTool(t, connect(t, perses), testCase.tool, testCase.args)
			if result.IsError {
				t.Fatalf("validation failure must be a successful tool result: %s", text)
			}
			if text != testCase.expected {
				t.Errorf("expected %s, got %s", testCase.expected, text)
			}
			if requests := perses.Received(); len(requests) != 0 {
				t.Errorf("expected no request, got %d", len(requests))
			}
		})
	}
}

func TestStringPayloadIsRejected(t *testing.T) {
	perses := fakePerses(http.StatusOK, nil)
	result, err := connect(t, perses).CallTool(context.Background(), &mcp.CallToolParams{
		Name: "perses_validate_dashboard", Arguments: map[string]any{"dashboard": dashboardJSON},
	})
	if err == nil && !result.IsError {
		t.Fatal("expected a dashboard given as a string to be rejected")
	}
	if requests := perses.Received(); len(requests) != 0 {
		t.Errorf("expected no request, got %d", len(requests))
	}
}

func rawJSON(document string) map[string]any {
	var value map[string]any
	if err := json.Unmarshal([]byte(document), &value); err != nil {
		panic(err)
	}
	return value
}

func findRequest(requests []tooltest.Request, method, path string) *tooltest.Request {
	for i := range requests {
		if requests[i].Method == method && requests[i].Path == path {
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
			name: "dashboard without project", tool: "perses_validate_dashboard",
			args: map[string]any{"dashboard": rawJSON(strings.Replace(dashboardJSON, `,"project":"test"`, "", 1))},
			path: "/api/validate/dashboards", expected: strings.Replace(dashboardJSON, `,"project":"test"`, "", 1), entity: &v1.Dashboard{},
		},
		{
			name: "project variable", tool: "perses_validate_variable", args: map[string]any{"variable": rawJSON(projectVarJSON)},
			path: "/api/validate/variables", expected: projectVarJSON, entity: &v1.Variable{},
		},
		{
			name: "global variable", tool: "perses_validate_variable", args: map[string]any{"variable": rawJSON(globalVarJSON)},
			path: "/api/validate/globalvariables", expected: globalVarJSON, entity: &v1.GlobalVariable{},
		},
		{
			name: "global variable with a lower-case kind", tool: "perses_validate_variable",
			args: map[string]any{"variable": rawJSON(strings.Replace(globalVarJSON, `"GlobalVariable"`, `"globalvariable"`, 1))},
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
			perses := fakePerses(http.StatusOK, nil)
			result, text := tooltest.CallTool(t, connect(t, perses), testCase.tool, testCase.args)
			if result.IsError || text != `{"valid":true}` {
				t.Fatalf("unexpected validation result: IsError=%v, %s", result.IsError, text)
			}
			request := findRequest(perses.Received(), http.MethodPost, testCase.path)
			if request == nil {
				t.Fatalf("no request to %s: %#v", testCase.path, perses.Received())
			}
			if request.Body != string(expectedBody) {
				t.Errorf("unexpected body %s; expected %s", request.Body, expectedBody)
			}
			if request.ContentType != "application/json" || request.Authorization != tooltest.Authorization {
				t.Errorf("unexpected request headers: %#v", request)
			}
		})
	}
}

func TestValidationServerErrors(t *testing.T) {
	dashboardWithPanel := `{"kind":"Dashboard","metadata":{"name":"cpu","project":"test"},"spec":{"duration":"1h",` +
		`"panels":{"p1":{"kind":"Panel","spec":{"display":{"name":"CPU"},"plugin":{"kind":"StatChart","spec":{"calculation":"average"}}}}},"layouts":[]}}`
	testCases := []struct {
		name   string
		tool   string
		args   map[string]any
		path   string
		status int
		body   string
		// expected is the result when Perses rejects the resource; empty when the tool must fail.
		expected string
		// message is the expected tool error when the resource could not be validated.
		message string
	}{
		{
			name: "dashboard rejected by a plugin schema", tool: "perses_validate_dashboard",
			args: map[string]any{"dashboard": rawJSON(dashboardWithPanel)},
			path: "POST /api/validate/dashboards", status: http.StatusBadRequest,
			body:     `{"message":"bad request: invalid panel p1: spec.calculation: conflicting values \"mean\" and \"average\""}`,
			expected: `{"valid":false,"error":"bad request: invalid panel p1: spec.calculation: conflicting values \"mean\" and \"average\""}`,
		},
		{
			name: "variable rejected by a plugin schema", tool: "perses_validate_variable", args: map[string]any{"variable": rawJSON(projectVarJSON)},
			path: "POST /api/validate/variables", status: http.StatusBadRequest,
			body:     `{"message":"invalid variable : spec.foo: field not allowed"}`,
			expected: `{"valid":false,"error":"invalid variable : spec.foo: field not allowed"}`,
		},
		{
			name: "dashboard server failure", tool: "perses_validate_dashboard", args: map[string]any{"dashboard": rawJSON(dashboardJSON)},
			path: "POST /api/validate/dashboards", status: http.StatusInternalServerError, body: `{"message":"boom"}`,
			message: "internal server error",
		},
		{
			name: "variable server failure", tool: "perses_validate_variable", args: map[string]any{"variable": rawJSON(globalVarJSON)},
			path: "POST /api/validate/globalvariables", status: http.StatusInternalServerError, body: `{"message":"boom"}`,
			message: "internal server error",
		},
		{
			name: "other client error", tool: "perses_validate_dashboard", args: map[string]any{"dashboard": rawJSON(dashboardJSON)},
			path: "POST /api/validate/dashboards", status: http.StatusTooManyRequests, body: `{"message":"too many requests"}`,
			message: "too many requests",
		},
		{
			name: "bad request without a Perses message", tool: "perses_validate_dashboard", args: map[string]any{"dashboard": rawJSON(dashboardJSON)},
			path: "POST /api/validate/dashboards", status: http.StatusBadRequest,
			body:    `<html><body><h1>400 Bad Request</h1></body></html>`,
			message: "StatusCode: 400",
		},
		{
			name: "bad request with an empty body", tool: "perses_validate_variable", args: map[string]any{"variable": rawJSON(projectVarJSON)},
			path: "POST /api/validate/variables", status: http.StatusBadRequest,
			message: "StatusCode: 400",
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			perses := fakePerses(http.StatusNotFound, map[string]fakeResponse{
				testCase.path: {status: testCase.status, body: testCase.body},
			})
			result, text := tooltest.CallTool(t, connect(t, perses), testCase.tool, testCase.args)
			if testCase.expected != "" {
				if result.IsError || text != testCase.expected {
					t.Errorf("expected %s, got IsError=%v, %s", testCase.expected, result.IsError, text)
				}
				return
			}
			if !result.IsError || !strings.Contains(text, testCase.message) {
				t.Errorf("expected the error of the Perses client unchanged, got %s", text)
			}
		})
	}
}

func TestValidateVariableRejectsOtherKindsWithoutRequest(t *testing.T) {
	testCases := []struct {
		name     string
		variable string
		expected string
	}{
		{
			name:     "missing kind",
			variable: `{"metadata":{"name":"test","project":"test"},"spec":{"kind":"TextVariable","spec":{"name":"test","value":"test"}}}`,
			expected: `{"valid":false,"error":"kind must be \"Variable\" or \"GlobalVariable\", got \"\""}`,
		},
		{
			name:     "dashboard kind",
			variable: strings.Replace(projectVarJSON, `"Variable"`, `"Dashboard"`, 1),
			expected: `{"valid":false,"error":"kind must be \"Variable\" or \"GlobalVariable\", got \"Dashboard\""}`,
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			perses := fakePerses(http.StatusOK, nil)
			result, text := tooltest.CallTool(t, connect(t, perses), "perses_validate_variable", map[string]any{"variable": rawJSON(testCase.variable)})
			if result.IsError || text != testCase.expected {
				t.Errorf("expected %s, got IsError=%v, %s", testCase.expected, result.IsError, text)
			}
			if requests := perses.Received(); len(requests) != 0 {
				t.Errorf("expected no request, got %d", len(requests))
			}
		})
	}
}

func TestValidationToolsMetadata(t *testing.T) {
	expected := []string{"perses_validate_dashboard", "perses_validate_variable"}
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
