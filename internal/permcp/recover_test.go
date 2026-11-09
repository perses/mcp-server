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

package permcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	v1 "github.com/perses/perses/pkg/model/api/v1"
)

type decodeInput struct {
	Dashboard json.RawMessage `json:"dashboard"`
}

// TestRecoverPanics decodes a dashboard with the Perses model like the dashboard tools do. A Grid item without
// content makes the model panic (nil JSON reference); the call must fail as a tool error and the server keep serving.
func TestRecoverPanics(t *testing.T) {
	server := mcp.NewServer(&mcp.Implementation{Name: "perses-mcp-server-test"}, nil)
	server.AddReceivingMiddleware(recoverPanics)
	tool := &mcp.Tool{
		Name: "decode_dashboard",
		InputSchema: &jsonschema.Schema{
			Type:       "object",
			Properties: map[string]*jsonschema.Schema{"dashboard": {Type: "object"}},
		},
	}
	mcp.AddTool(server, tool, func(_ context.Context, _ *mcp.CallToolRequest, input decodeInput) (*mcp.CallToolResult, any, error) {
		dash := &v1.Dashboard{}
		if err := json.Unmarshal(input.Dashboard, dash); err != nil {
			return nil, nil, err
		}
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: dash.Metadata.Name}}}, nil, nil
	})

	ctx := context.Background()
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = serverSession.Close() })
	session, err := mcp.NewClient(&mcp.Implementation{Name: "test-client"}, nil).Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })

	call := func(dashboard string) (string, bool) {
		t.Helper()
		result, err := session.CallTool(ctx, &mcp.CallToolParams{
			Name:      "decode_dashboard",
			Arguments: map[string]any{"dashboard": json.RawMessage(dashboard)},
		})
		if err != nil {
			t.Fatalf("protocol error: %v", err)
		}
		return result.Content[0].(*mcp.TextContent).Text, result.IsError
	}

	text, isError := call(`{"kind":"Dashboard","metadata":{"name":"d","project":"p"},"spec":{"duration":"1h","layouts":[{"kind":"Grid","spec":{"items":[{"x":0,"y":0,"width":1,"height":1}]}}]}}`)
	if !isError || !strings.HasPrefix(text, "internal error: ") {
		t.Fatalf("expected an internal tool error, got isError=%v: %s", isError, text)
	}

	text, isError = call(`{"kind":"Dashboard","metadata":{"name":"d","project":"p"},"spec":{"duration":"1h"}}`)
	if isError || text != "d" {
		t.Fatalf("expected the server to keep serving, got isError=%v: %s", isError, text)
	}
}
