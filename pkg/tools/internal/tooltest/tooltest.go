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

// Package tooltest helps testing MCP tools end to end: it serves a fake Perses API, connects an MCP client to the
// tools through an in-memory transport and calls them.
package tooltest

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/perses/mcp-server/pkg/tools"
	apiClient "github.com/perses/perses/pkg/client/api"
	"github.com/perses/perses/pkg/client/perseshttp"
	"github.com/perses/spec/go/common"
)

// Authorization is the Authorization header sent by the clients of NewClient.
const Authorization = "Bearer test-token"

// Request is a request received by Perses.
type Request struct {
	Method        string
	Path          string
	RawQuery      string
	Body          string
	ContentType   string
	Authorization string
}

// Perses is a fake Perses API: it records the requests it receives and lets Handler answer them.
type Perses struct {
	Handler  http.Handler
	mutex    sync.Mutex
	requests []Request
}

func (p *Perses) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	p.mutex.Lock()
	p.requests = append(p.requests, Request{
		Method:        r.Method,
		Path:          r.URL.Path,
		RawQuery:      r.URL.RawQuery,
		Body:          string(body),
		ContentType:   r.Header.Get("Content-Type"),
		Authorization: r.Header.Get("Authorization"),
	})
	p.mutex.Unlock()
	p.Handler.ServeHTTP(w, r)
}

// Received returns the requests received so far.
func (p *Perses) Received() []Request {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	return append([]Request(nil), p.requests...)
}

// Count returns the number of requests received for the method and path.
func (p *Perses) Count(method, path string) int {
	count := 0
	for _, request := range p.Received() {
		if request.Method == method && request.Path == path {
			count++
		}
	}
	return count
}

// NewClient serves perses on a test HTTP server and returns a Perses client sending Authorization to it.
func NewClient(t testing.TB, perses http.Handler) apiClient.ClientInterface {
	t.Helper()
	httpServer := httptest.NewServer(perses)
	t.Cleanup(httpServer.Close)
	return apiClient.NewWithClient(&perseshttp.RESTClient{
		BaseURL: common.MustParseURL(httpServer.URL),
		Client:  httpServer.Client(),
		Headers: map[string]string{"Authorization": Authorization},
	})
}

// Connect registers the tools on an in-memory MCP server and returns the session of a client connected to it.
func Connect(t testing.TB, toolset []*tools.Tool) *mcp.ClientSession {
	t.Helper()
	server := mcp.NewServer(&mcp.Implementation{Name: "test-server"}, nil)
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

// CallTool calls a tool and returns its result and its text contents joined by newlines. Nil arguments are sent as
// an empty object. A protocol error fails the test, as tool errors are reported in the result.
func CallTool(t testing.TB, session *mcp.ClientSession, name string, arguments map[string]any) (*mcp.CallToolResult, string) {
	t.Helper()
	if arguments == nil {
		arguments = map[string]any{}
	}
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
