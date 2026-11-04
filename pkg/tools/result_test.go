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
	"encoding/json"
	"math"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func singleText(t *testing.T, result *mcp.CallToolResult) string {
	t.Helper()
	if result == nil {
		t.Fatal("result is nil")
	}
	if result.IsError {
		t.Error("IsError = true, want false")
	}
	if len(result.Content) != 1 {
		t.Fatalf("len(Content) = %d, want 1", len(result.Content))
	}
	text, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("Content[0] is %T, want *mcp.TextContent", result.Content[0])
	}
	return text.Text
}

func TestTextResult(t *testing.T) {
	got := singleText(t, TextResult("Dashboard 'demo' deleted successfully"))
	if want := "Dashboard 'demo' deleted successfully"; got != want {
		t.Errorf("text = %q, want %q", got, want)
	}
}

func TestJSONResult(t *testing.T) {
	tests := []struct {
		name  string
		value any
		want  string
	}{
		{
			name:  "struct is compact JSON",
			value: struct{ Name, Kind string }{Name: "demo", Kind: "Project"},
			want:  `{"Name":"demo","Kind":"Project"}`,
		},
		{
			name:  "PromQL is not HTML-escaped",
			value: map[string]string{"query": `rate(http_requests_total[5m]) > 0 && up < 1`},
			want:  `{"query":"rate(http_requests_total[5m]) > 0 && up < 1"}`,
		},
		{
			name:  "raw message is compacted",
			value: json.RawMessage("{\n  \"a\": \"<b>\"\n}"),
			want:  `{"a":"<b>"}`,
		},
		{
			name:  "slice",
			value: []int{1, 2},
			want:  `[1,2]`,
		},
		{
			name:  "nil",
			value: nil,
			want:  `null`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := JSONResult(tt.value)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got := singleText(t, result); got != tt.want {
				t.Errorf("text = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestJSONResultError(t *testing.T) {
	result, err := JSONResult(math.Inf(1))
	if err == nil {
		t.Fatal("expected an error for an unsupported value")
	}
	if result != nil {
		t.Errorf("result = %v, want nil", result)
	}
}
