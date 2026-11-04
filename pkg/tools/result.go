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

package tools

import (
	"bytes"
	"encoding/json"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// TextResult returns a tool result with a single text content block.
func TextResult(text string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: text},
		},
	}
}

// JSONResult encodes v as compact JSON and returns it as a single text content block.
//
// HTML escaping is disabled on purpose. By default, encoding/json turns '<', '>' and '&'
// into \u003c, \u003e and \u0026. Queries rely on these characters (e.g. PromQL
// `rate(x[5m]) > 0 and y < 1`, or '&' in label values and URLs), and the escaped form is
// harder for models to read and costs extra tokens.
func JSONResult(v any) (*mcp.CallToolResult, error) {
	text, err := marshalJSON(v)
	if err != nil {
		return nil, err
	}
	return TextResult(text), nil
}

// marshalJSON encodes v as compact JSON without HTML escaping. See JSONResult.
func marshalJSON(v any) (string, error) {
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(v); err != nil {
		return "", err
	}
	return string(bytes.TrimSuffix(buf.Bytes(), []byte("\n"))), nil
}
