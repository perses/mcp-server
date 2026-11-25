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

// Package ui embeds the MCP Apps views built from ui/mcp-apps (make build-mcp-apps). The views are not committed: a
// server built without them doesn't have the tools that display them.
package ui

import "embed"

// The placeholder keeps the pattern valid when the views are not built.
//
//go:embed mcp-apps/dist/*.html
var views embed.FS

// PanelsHTML returns the view of perses_render_panels, a single HTML file with its scripts and styles, and false
// when the views are not built.
func PanelsHTML() (string, bool) {
	html, err := views.ReadFile("mcp-apps/dist/panels.html")
	if err != nil {
		return "", false
	}
	return string(html), true
}
