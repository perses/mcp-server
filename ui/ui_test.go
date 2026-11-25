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

package ui

import (
	"os"
	"strings"
	"testing"
)

// requireViewsEnv makes the test fail when the views are not built: set by the builds that must embed them.
const requireViewsEnv = "PERSES_MCP_REQUIRE_VIEWS"

func TestPanelsHTML(t *testing.T) {
	html, ok := PanelsHTML()
	if !ok {
		if os.Getenv(requireViewsEnv) != "" {
			t.Fatal("the MCP Apps views are not built: run 'make build-mcp-apps'")
		}
		if html != "" {
			t.Errorf("PanelsHTML() = %q without the view, want an empty string", html)
		}
		t.Skip("the MCP Apps views are not built")
	}
	if !strings.Contains(html, "<html") || !strings.Contains(html, "<script") {
		t.Errorf("PanelsHTML() is not the built view (%d bytes)", len(html))
	}
}
