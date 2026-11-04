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
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	apiclient "github.com/perses/perses/pkg/client/api"
	"github.com/perses/perses/pkg/client/perseshttp"
	"github.com/perses/spec/go/common"
)

const testAuthorization = "Bearer test"

const dashboardJSON = `{"kind":"Dashboard","metadata":{"name":"test","project":"test"},"spec":{"duration":"1h","panels":{},"layouts":[]}}`

type capturedRequest struct {
	method        string
	path          string
	body          string
	contentType   string
	authorization string
	query         string
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
		query:         r.URL.RawQuery,
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
