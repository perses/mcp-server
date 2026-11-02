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

// Package proxytest provides the shared test cases of the datasource proxy query tools.
package proxytest

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/perses/mcp-server/pkg/tools"
	apiclient "github.com/perses/perses/pkg/client/api"
	apiClient "github.com/perses/perses/pkg/client/api/v1"
	"github.com/perses/perses/pkg/client/perseshttp"
	"github.com/perses/spec/go/common"
)

// QueryTool describes the proxy query tool under test.
type QueryTool struct {
	// Tools returns the tools of the resource that exposes the query tool.
	Tools func(client apiClient.ClientInterface) []*tools.Tool
	// Name is the name of the query tool.
	Name string
	// Arguments are the arguments of the tool call.
	Arguments map[string]any
	// WantPath is the Perses proxy path the tool must call.
	WantPath string
	// WantErrorContext is the resource context the tool error must contain.
	WantErrorContext string
}

type queryCase struct {
	name        string
	contentType string
	status      int
	body        string
	wantError   bool
	want        []string
}

// RunQueryResponseTests calls the query tool against a fake Perses proxy and checks that the response body is returned
// unchanged on success and as a tool error on an HTTP error status.
func RunQueryResponseTests(t *testing.T, qt QueryTool) {
	t.Helper()
	largeBody := strings.Repeat("a", 1<<20+1024)
	tests := []queryCase{
		{
			name:        "JSON body is returned unchanged",
			contentType: "application/json",
			status:      http.StatusOK,
			body:        `{"status":"success","data":{"resultType":"vector","result":[]}}`,
			want:        []string{`{"status":"success","data":{"resultType":"vector","result":[]}}`},
		},
		{
			name:        "JSON body with a charset is returned unchanged",
			contentType: "application/json; charset=utf-8",
			status:      http.StatusOK,
			body:        `{"status":"success","data":["up"]}`,
			want:        []string{`{"status":"success","data":["up"]}`},
		},
		{
			name:        "invalid JSON is returned unchanged",
			contentType: "application/json",
			status:      http.StatusOK,
			body:        `{"status":`,
			want:        []string{`{"status":`},
		},
		{
			name:        "text body is returned unchanged",
			contentType: "text/plain; charset=utf-8",
			status:      http.StatusOK,
			body:        "Prometheus Server is Ready.\n",
			want:        []string{"Prometheus Server is Ready.\n"},
		},
		{
			name:        "body over 1 MiB is truncated with a marker",
			contentType: "text/plain; charset=utf-8",
			status:      http.StatusOK,
			body:        largeBody,
			want:        []string{largeBody[:1<<20] + "\n[response truncated: exceeded 1 MiB limit]"},
		},
		{
			name:        "error status is a tool error with the status and the body",
			contentType: "application/json",
			status:      http.StatusBadRequest,
			body:        `{"status":"error","errorType":"bad_data","error":"invalid parameter \"query\""}`,
			wantError:   true,
			want:        []string{"HTTP 400 Bad Request", `"errorType":"bad_data"`, qt.WantErrorContext},
		},
		{
			name:        "Perses proxy error is a tool error",
			contentType: "application/json",
			status:      http.StatusInternalServerError,
			body:        `{"message":"dial tcp: connection refused"}`,
			wantError:   true,
			want:        []string{"HTTP 500 Internal Server Error", "connection refused", qt.WantErrorContext},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runQueryCase(t, qt, tt)
		})
	}
}

func runQueryCase(t *testing.T, qt QueryTool, tt queryCase) {
	t.Helper()
	var gotPath string
	httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", tt.contentType)
		w.Header().Set("X-Response-Header", "must-not-appear")
		w.WriteHeader(tt.status)
		_, _ = w.Write([]byte(tt.body))
	}))
	defer httpServer.Close()
	client := apiclient.NewWithClient(&perseshttp.RESTClient{
		BaseURL: common.MustParseURL(httpServer.URL),
		Client:  httpServer.Client(),
	})

	server := mcp.NewServer(&mcp.Implementation{Name: "test-server"}, nil)
	for _, tool := range qt.Tools(client.V1()) {
		tool.RegisterWith(server)
	}
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(context.Background(), serverTransport, nil)
	if err != nil {
		t.Fatalf("connecting the server: %v", err)
	}
	defer func() { _ = serverSession.Close() }()
	session, err := mcp.NewClient(&mcp.Implementation{Name: "test-client"}, nil).Connect(context.Background(), clientTransport, nil)
	if err != nil {
		t.Fatalf("connecting the client: %v", err)
	}
	defer func() { _ = session.Close() }()

	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: qt.Name, Arguments: qt.Arguments})
	if err != nil {
		t.Fatalf("calling the tool: %v", err)
	}
	if gotPath != qt.WantPath {
		t.Errorf("proxy path = %q, want %q", gotPath, qt.WantPath)
	}
	if result.IsError != tt.wantError {
		t.Errorf("isError = %v, want %v", result.IsError, tt.wantError)
	}
	if len(result.Content) != 1 {
		t.Fatalf("got %d contents, want 1", len(result.Content))
	}
	text := result.Content[0].(*mcp.TextContent).Text
	if !tt.wantError && text != tt.want[0] {
		t.Errorf("text = %q, want %q", shorten(text), shorten(tt.want[0]))
	}
	for _, want := range tt.want {
		if !strings.Contains(text, want) {
			t.Errorf("text %q does not contain %q", shorten(text), shorten(want))
		}
	}
	for _, unwanted := range []string{"X-Response-Header", "must-not-appear", "status_code"} {
		if strings.Contains(text, unwanted) {
			t.Errorf("text %q contains %q", shorten(text), unwanted)
		}
	}
}

// shorten keeps failure messages readable for large bodies.
func shorten(s string) string {
	const limit = 200
	if len(s) <= limit {
		return s
	}
	return s[:limit/2] + "..." + s[len(s)-limit/2:]
}
