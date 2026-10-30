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

const dashboardJSON = `{"kind":"Dashboard","metadata":{"name":"test","project":"test"},"spec":{"duration":"1h","panels":{},"layouts":[]}}`

type capturedRequest struct {
	method        string
	path          string
	body          string
	contentType   string
	authorization string
}

type fakePerses struct {
	mutex    sync.Mutex
	requests []capturedRequest
	status   int
	body     string
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
	w.WriteHeader(f.status)
	_, _ = w.Write([]byte(f.body))
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
		Headers: map[string]string{"Authorization": "Bearer test"},
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

func callTool(t *testing.T, session *mcp.ClientSession, name, field, payload string) (*mcp.CallToolResult, string) {
	t.Helper()
	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name: name, Arguments: map[string]any{field: payload},
	})
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

func TestValidationRequests(t *testing.T) {
	testCases := []struct {
		name    string
		field   string
		path    string
		payload string
		entity  any
	}{
		{
			name: "perses_validate_dashboard", field: "dashboard", path: "/api/validate/dashboards",
			payload: dashboardJSON, entity: &v1.Dashboard{},
		},
		{
			name: "perses_validate_datasource", field: "datasource", path: "/api/validate/datasources",
			payload: `{"kind":"Datasource","metadata":{"name":"test","project":"test"},"spec":{"plugin":{"kind":"PrometheusDatasource","spec":{"directUrl":"http://localhost:9090"}}}}`,
			entity:  &v1.Datasource{},
		},
		{
			name: "perses_validate_global_datasource", field: "datasource", path: "/api/validate/globaldatasources",
			payload: `{"kind":"GlobalDatasource","metadata":{"name":"test"},"spec":{"plugin":{"kind":"PrometheusDatasource","spec":{"directUrl":"http://localhost:9090"}}}}`,
			entity:  &v1.GlobalDatasource{},
		},
		{
			name: "perses_validate_variable", field: "variable", path: "/api/validate/variables",
			payload: `{"kind":"Variable","metadata":{"name":"test","project":"test"},"spec":{"kind":"TextVariable","spec":{"value":"test"}}}`,
			entity:  &v1.Variable{},
		},
		{
			name: "perses_validate_global_variable", field: "variable", path: "/api/validate/globalvariables",
			payload: `{"kind":"GlobalVariable","metadata":{"name":"test"},"spec":{"kind":"TextVariable","spec":{"value":"test"}}}`,
			entity:  &v1.GlobalVariable{},
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			if err := json.Unmarshal([]byte(testCase.payload), testCase.entity); err != nil {
				t.Fatalf("invalid test fixture: %v", err)
			}
			expectedBody, err := json.Marshal(testCase.entity)
			if err != nil {
				t.Fatal(err)
			}
			perses := &fakePerses{status: http.StatusOK}
			result, text := callTool(t, connect(t, perses), testCase.name, testCase.field, testCase.payload)
			if result.IsError || text != `{"valid":true}` {
				t.Fatalf("unexpected validation result: IsError=%v, %s", result.IsError, text)
			}
			requests := perses.received()
			if len(requests) != 1 {
				t.Fatalf("expected 1 request, got %d", len(requests))
			}
			request := requests[0]
			if request.method != http.MethodPost || request.path != testCase.path || request.body != string(expectedBody) {
				t.Errorf("unexpected request: %#v; expected body %s", request, expectedBody)
			}
			if request.contentType != "application/json" || request.authorization != "Bearer test" {
				t.Errorf("unexpected request headers: %#v", request)
			}
		})
	}
}

func TestValidationResults(t *testing.T) {
	testCases := []struct {
		name    string
		status  int
		body    string
		errors  string
		isError bool
	}{
		{name: "valid", status: http.StatusOK},
		{name: "invalid", status: http.StatusBadRequest, body: `{"message":"spec.kind: invalid"}`, errors: "spec.kind: invalid"},
		{name: "server error", status: http.StatusInternalServerError, body: `{"message":"server failure"}`, isError: true},
		{name: "not found", status: http.StatusNotFound, body: `{"message":"not found"}`, isError: true},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			perses := &fakePerses{status: testCase.status, body: testCase.body}
			result, text := callTool(t, connect(t, perses), "perses_validate_dashboard", "dashboard", dashboardJSON)
			if result.IsError != testCase.isError {
				t.Fatalf("expected IsError=%v, got %v: %s", testCase.isError, result.IsError, text)
			}
			if testCase.isError {
				if !strings.Contains(text, "error validating a dashboard:") {
					t.Errorf("unexpected error: %s", text)
				}
				return
			}
			var decoded validationResult
			if err := json.Unmarshal([]byte(text), &decoded); err != nil {
				t.Fatal(err)
			}
			if decoded.Valid != (testCase.status == http.StatusOK) || decoded.Errors != testCase.errors {
				t.Errorf("unexpected validation result: %s", text)
			}
		})
	}
}

func TestRejectsInvalidPayloadWithoutRequest(t *testing.T) {
	for _, payload := range []string{`{"kind":`, `{"kind":"Datasource","metadata":{"name":"test","project":"test"},"spec":{"duration":"1h"}}`} {
		t.Run(payload, func(t *testing.T) {
			perses := &fakePerses{status: http.StatusOK}
			result, text := callTool(t, connect(t, perses), "perses_validate_dashboard", "dashboard", payload)
			if result.IsError {
				t.Fatalf("validation failure must be a successful tool result: %s", text)
			}
			var decoded validationResult
			if err := json.Unmarshal([]byte(text), &decoded); err != nil {
				t.Fatal(err)
			}
			if decoded.Valid || decoded.Errors == "" {
				t.Errorf("expected validation error, got %s", text)
			}
			if requests := perses.received(); len(requests) != 0 {
				t.Errorf("expected no request, got %d", len(requests))
			}
		})
	}
}

func TestValidationToolsMetadata(t *testing.T) {
	subjects := map[string]string{
		"perses_validate_dashboard":         "a dashboard",
		"perses_validate_datasource":        "a project datasource",
		"perses_validate_global_datasource": "a global datasource",
		"perses_validate_variable":          "a project variable",
		"perses_validate_global_variable":   "a global variable",
	}
	client := apiclient.NewWithClient(&perseshttp.RESTClient{})
	toolset := New(client).GetTools()
	if len(toolset) != len(subjects) {
		t.Fatalf("expected %d tools, got %d", len(subjects), len(toolset))
	}
	for _, tool := range toolset {
		name := tool.MCPTool.Name
		subject, ok := subjects[name]
		if !ok {
			t.Fatalf("unexpected tool: %s", name)
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
		if annotations.Title != "Validates "+subject+" against Perses schemas" {
			t.Errorf("%s: unexpected title %q", name, annotations.Title)
		}
		for _, fragment := range []string{"Validate " + subject, `{"valid": true}`, `{"valid": false, "errors": "..."}`, "does not create or modify anything"} {
			if !strings.Contains(tool.MCPTool.Description, fragment) {
				t.Errorf("%s: description missing %q", name, fragment)
			}
		}
	}
}
