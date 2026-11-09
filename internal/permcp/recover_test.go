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
	"errors"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// toolResultText returns the text of a tool result with a single text content, and whether it is an error.
func toolResultText(t *testing.T, result mcp.Result) (string, bool) {
	t.Helper()
	toolResult, ok := result.(*mcp.CallToolResult)
	if !ok {
		t.Fatalf("expected a *mcp.CallToolResult, got %T", result)
	}
	if len(toolResult.Content) != 1 {
		t.Fatalf("expected one content, got %d", len(toolResult.Content))
	}
	text, ok := toolResult.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("expected a *mcp.TextContent, got %T", toolResult.Content[0])
	}
	return text.Text, toolResult.IsError
}

func TestRecoverPanicsHandler(t *testing.T) {
	passedResult := &mcp.CallToolResult{}
	panics := func(context.Context, string, mcp.Request) (mcp.Result, error) { panic("boom") }
	returns := func(context.Context, string, mcp.Request) (mcp.Result, error) {
		return passedResult, errors.New("not found")
	}
	tests := []struct {
		name          string
		method        string
		next          mcp.MethodHandler
		wantToolError string
		wantResult    mcp.Result
		wantErr       string
	}{
		{name: "tool call panic", method: "tools/call", next: panics, wantToolError: "internal error: boom"},
		{name: "other method panic", method: "resources/read", next: panics, wantErr: "internal error: boom"},
		{name: "no panic", method: "tools/call", next: returns, wantResult: passedResult, wantErr: "not found"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := recoverPanics(test.next)(t.Context(), test.method, nil)
			if test.wantToolError != "" {
				if err != nil {
					t.Fatalf("expected no error, got %v", err)
				}
				if text, isError := toolResultText(t, result); !isError || !strings.Contains(text, test.wantToolError) {
					t.Fatalf("expected a tool error containing %q, got isError=%v: %s", test.wantToolError, isError, text)
				}
				return
			}
			if result != test.wantResult {
				t.Fatalf("expected result %v, got %v", test.wantResult, result)
			}
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("expected an error containing %q, got %v", test.wantErr, err)
			}
		})
	}
}

type panicInput struct {
	Panic bool `json:"panic"`
}

// TestRecoverPanics checks end to end that a panicking tool fails as a tool error and the server keeps serving.
func TestRecoverPanics(t *testing.T) {
	server := mcp.NewServer(&mcp.Implementation{Name: "perses-mcp-server-test"}, nil)
	server.AddReceivingMiddleware(recoverPanics)
	mcp.AddTool(server, &mcp.Tool{Name: "maybe_panic"}, func(_ context.Context, _ *mcp.CallToolRequest, input panicInput) (*mcp.CallToolResult, any, error) {
		if input.Panic {
			panic("boom")
		}
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "ok"}}}, nil, nil
	})

	ctx := t.Context()
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

	call := func(shouldPanic bool) (string, bool) {
		t.Helper()
		result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "maybe_panic", Arguments: panicInput{Panic: shouldPanic}})
		if err != nil {
			t.Fatalf("protocol error: %v", err)
		}
		return toolResultText(t, result)
	}

	if text, isError := call(true); !isError || !strings.HasPrefix(text, "internal error: ") {
		t.Fatalf("expected an internal tool error, got isError=%v: %s", isError, text)
	}
	if text, isError := call(false); isError || text != "ok" {
		t.Fatalf("expected the server to keep serving, got isError=%v: %s", isError, text)
	}
}
