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

package mcpapp

import (
	"encoding/json"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestToolMeta(t *testing.T) {
	got, err := json.Marshal(ToolMeta("ui://perses/panels.html"))
	if err != nil {
		t.Fatal(err)
	}
	if want := `{"ui":{"resourceUri":"ui://perses/panels.html"}}`; string(got) != want {
		t.Errorf("ToolMeta = %s, want %s", got, want)
	}
	got, err = json.Marshal(ToolMeta("", VisibilityApp))
	if err != nil {
		t.Fatal(err)
	}
	if want := `{"ui":{"visibility":["app"]}}`; string(got) != want {
		t.Errorf("ToolMeta = %s, want %s", got, want)
	}
}

func TestIsAppOnly(t *testing.T) {
	tests := []struct {
		name string
		meta mcp.Meta
		want bool
	}{
		{name: "no meta", want: false},
		{name: "default visibility", meta: ToolMeta("ui://x"), want: false},
		{name: "app only", meta: ToolMeta("", VisibilityApp), want: true},
		{name: "model and app", meta: ToolMeta("", VisibilityModel, VisibilityApp), want: false},
		{name: "decoded app only", meta: mcp.Meta{"ui": map[string]any{"visibility": []any{"app"}}}, want: true},
		{name: "decoded model", meta: mcp.Meta{"ui": map[string]any{"visibility": []any{"model"}}}, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsAppOnly(&mcp.Tool{Meta: tt.meta}); got != tt.want {
				t.Errorf("IsAppOnly() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSupported(t *testing.T) {
	tests := []struct {
		name         string
		capabilities *mcp.ClientCapabilities
		want         bool
	}{
		{name: "no capabilities", want: false},
		{name: "no extension", capabilities: &mcp.ClientCapabilities{}, want: false},
		{name: "other extension", capabilities: &mcp.ClientCapabilities{Extensions: map[string]any{"io.example/other": map[string]any{}}}, want: false},
		{name: "extension", capabilities: &mcp.ClientCapabilities{Extensions: map[string]any{ExtensionID: map[string]any{"mimeTypes": []any{ResourceMIMEType}}}}, want: true},
		{name: "extension without mime types", capabilities: &mcp.ClientCapabilities{Extensions: map[string]any{ExtensionID: map[string]any{}}}, want: true},
		{name: "other mime types", capabilities: &mcp.ClientCapabilities{Extensions: map[string]any{ExtensionID: map[string]any{"mimeTypes": []any{"text/uri-list"}}}}, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Supported(tt.capabilities); got != tt.want {
				t.Errorf("Supported() = %v, want %v", got, tt.want)
			}
		})
	}
}
