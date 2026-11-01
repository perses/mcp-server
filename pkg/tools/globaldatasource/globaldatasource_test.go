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

package globaldatasource

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	apiclient "github.com/perses/perses/pkg/client/api"
	"github.com/perses/perses/pkg/client/perseshttp"
	"github.com/perses/spec/go/common"
)

func TestQueryResponse(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		status      int
		body        string
		wantError   bool
		want        []string
	}{
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
			body:        `{"truncated":`,
			want:        []string{`{"truncated":`},
		},
		{
			name:        "text body is returned unchanged",
			contentType: "text/plain; charset=utf-8",
			status:      http.StatusOK,
			body:        "Prometheus Server is Ready.\n",
			want:        []string{"Prometheus Server is Ready.\n"},
		},
		{
			name:        "error status is a tool error with the status and the body",
			contentType: "application/json",
			status:      http.StatusBadRequest,
			body:        `{"status":"error","errorType":"bad_data","error":"invalid parameter \"query\""}`,
			wantError:   true,
			want:        []string{"HTTP 400 Bad Request", `"errorType":"bad_data"`, "global datasource 'prom'"},
		},
		{
			name:        "Perses proxy error is a tool error",
			contentType: "application/json",
			status:      http.StatusInternalServerError,
			body:        `{"message":"dial tcp: connection refused"}`,
			wantError:   true,
			want:        []string{"HTTP 500 Internal Server Error", "connection refused"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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
			for _, tool := range New(client.V1()).GetTools() {
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

			result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "perses_query_global_datasource", Arguments: map[string]any{"datasource_name": "prom", "path": "/api/v1/query"}})
			if err != nil {
				t.Fatalf("calling the tool: %v", err)
			}
			if gotPath != "/proxy/globaldatasources/prom/api/v1/query" {
				t.Errorf("proxy path = %q, want %q", gotPath, "/proxy/globaldatasources/prom/api/v1/query")
			}
			if result.IsError != tt.wantError {
				t.Errorf("isError = %v, want %v", result.IsError, tt.wantError)
			}
			if len(result.Content) != 1 {
				t.Fatalf("got %d contents, want 1", len(result.Content))
			}
			text := result.Content[0].(*mcp.TextContent).Text
			if !tt.wantError && text != tt.want[0] {
				t.Errorf("text = %q, want %q", text, tt.want[0])
			}
			for _, want := range tt.want {
				if !strings.Contains(text, want) {
					t.Errorf("text %q does not contain %q", text, want)
				}
			}
			for _, unwanted := range []string{"X-Response-Header", "must-not-appear", "status_code"} {
				if strings.Contains(text, unwanted) {
					t.Errorf("text %q contains %q", text, unwanted)
				}
			}
		})
	}
}
