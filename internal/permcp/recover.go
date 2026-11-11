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
	"fmt"
	"runtime/debug"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/sirupsen/logrus"
)

// recoverPanics is a receiving middleware turning a panic while handling a request into an error, so that one bad
// call cannot crash the server. A panic in a tool call is returned as a tool error, other requests fail with a
// JSON-RPC error. The MCP SDK does not recover panics of the handlers.
func recoverPanics(next mcp.MethodHandler) mcp.MethodHandler {
	return func(ctx context.Context, method string, req mcp.Request) (result mcp.Result, err error) {
		defer func() {
			r := recover()
			if r == nil {
				return
			}
			fields := logrus.Fields{"method": method, "panic": r, "stack": string(debug.Stack())}
			if call, ok := req.(*mcp.CallToolRequest); ok && call.Params != nil {
				fields["tool"] = call.Params.Name
			}
			logrus.WithFields(fields).Error("recovered from a panic while handling a request")
			panicErr := fmt.Errorf("internal error: %v", r)
			if method == "tools/call" {
				toolResult := &mcp.CallToolResult{}
				toolResult.SetError(panicErr)
				result, err = toolResult, nil
				return
			}
			result, err = nil, panicErr
		}()
		return next(ctx, method, req)
	}
}
