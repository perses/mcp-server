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
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/perses/mcp-server/pkg/tools"
	apiclient "github.com/perses/perses/pkg/client/api"
	"github.com/perses/perses/pkg/client/perseshttp"
	v1 "github.com/perses/perses/pkg/model/api/v1"
	"github.com/perses/spec/go/common"
)

const testAuthorization = "Bearer test"

const (
	dashboardJSON  = `{"kind":"Dashboard","metadata":{"name":"test","project":"test"},"spec":{"duration":"1h","panels":{},"layouts":[]}}`
	projectVarJSON = `{"kind":"Variable","metadata":{"name":"test","project":"test"},"spec":{"kind":"TextVariable","spec":{"name":"test","value":"test"}}}`
	globalVarJSON  = `{"kind":"GlobalVariable","metadata":{"name":"test"},"spec":{"kind":"TextVariable","spec":{"name":"test","value":"test"}}}`
)

type capturedRequest struct {
	method        string
	path          string
	body          string
	contentType   string
	authorization string
}

type fakeResponse struct {
	status int
	body   string
}

type fakePerses struct {
	mutex    sync.Mutex
	requests []capturedRequest
	status   int
	body     string
	// routes overrides the response for "METHOD /path".
	routes map[string]fakeResponse
}

func (f *fakePerses) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	f.mutex.Lock()
	f.requests = append(f.requests, capturedRequest{
		method:        r.Method,
		path:          r.URL.Path,
		body:          string(body),
		contentType:   r.Header.Get("Content-Type"),
		authorization: r.Header.Get("Authorization"),
	})
	f.mutex.Unlock()
	response, ok := f.routes[r.Method+" "+r.URL.Path]
	if !ok {
		response = fakeResponse{status: f.status, body: f.body}
	}
	w.WriteHeader(response.status)
	_, _ = w.Write([]byte(response.body))
}

func (f *fakePerses) received() []capturedRequest {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	return append([]capturedRequest(nil), f.requests...)
}

func connect(t *testing.T, perses http.Handler) *mcp.ClientSession {
	t.Helper()
	httpServer := httptest.NewServer(perses)
	t.Cleanup(httpServer.Close)
	client := apiclient.NewWithClient(&perseshttp.RESTClient{
		BaseURL: common.MustParseURL(httpServer.URL),
		Client:  httpServer.Client(),
		Headers: map[string]string{"Authorization": testAuthorization},
	})
	server := mcp.NewServer(&mcp.Implementation{Name: "test-server"}, nil)
	for _, tool := range New(client).GetTools() {
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

func TestRejectsInvalidPayloadWithoutRequest(t *testing.T) {
	perses := &fakePerses{status: http.StatusOK}
	session := connect(t, perses)
	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "perses_validate_dashboard",
		Arguments: map[string]any{"dashboard": rawJSON(`{"kind":"Datasource","metadata":{"name":"test","project":"test"},"spec":{"duration":"1h"}}`)},
	})
	if err != nil {
		t.Fatal(err)
	}
	text := result.Content[0].(*mcp.TextContent).Text
	if result.IsError {
		t.Fatalf("validation failure must be a successful tool result: %s", text)
	}
	if expected := `{"valid":false,"error":"invalid kind: \"Datasource\" for a Dashboard type"}`; text != expected {
		t.Errorf("expected %s, got %s", expected, text)
	}
	if requests := perses.received(); len(requests) != 0 {
		t.Errorf("expected no request, got %d", len(requests))
	}
}

func TestStringPayloadIsRejected(t *testing.T) {
	perses := &fakePerses{status: http.StatusOK}
	result, err := connect(t, perses).CallTool(context.Background(), &mcp.CallToolParams{
		Name: "perses_validate_dashboard", Arguments: map[string]any{"dashboard": dashboardJSON},
	})
	if err == nil && !result.IsError {
		t.Fatal("expected a dashboard given as a string to be rejected")
	}
	if requests := perses.received(); len(requests) != 0 {
		t.Errorf("expected no request, got %d", len(requests))
	}
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
			perses := &fakePerses{status: http.StatusOK}
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
			name: "variable plugin field", tool: "perses_validate_variable", args: map[string]any{"variable": rawJSON(projectVarJSON)},
			path: "POST /api/validate/variables", status: http.StatusBadRequest,
			body:    `{"message":"invalid variable : spec.foo: field not allowed"}`,
			message: "invalid variable : spec.foo: field not allowed",
		},
		{
			name: "server failure", tool: "perses_validate_variable", args: map[string]any{"variable": rawJSON(globalVarJSON)},
			path: "POST /api/validate/globalvariables", status: http.StatusInternalServerError, body: `{"message":"boom"}`,
			message: "internal server error",
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			perses := &fakePerses{status: http.StatusNotFound, routes: map[string]fakeResponse{
				testCase.path: {status: testCase.status, body: testCase.body},
			}}
			result, text := call(t, connect(t, perses), testCase.tool, testCase.args)
			if !result.IsError || !strings.Contains(text, testCase.message) {
				t.Errorf("expected the error of the Perses client unchanged, got %s", text)
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
