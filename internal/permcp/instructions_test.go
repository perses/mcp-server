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
	"flag"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"testing"

	apiclient "github.com/perses/perses/pkg/client/api"
	"github.com/perses/perses/pkg/client/perseshttp"
	"github.com/perses/spec/go/common"

	"github.com/perses/mcp-server/pkg/tools"
)

var updateGolden = flag.Bool("update", false, "update the golden files of the instructions")

// toolNameRegexp matches the tool names of the instructions: buildInstructions always formats them as code.
var toolNameRegexp = regexp.MustCompile("`(perses_[a-z_]+)`")

// testPanelsView stands for the view of perses_render_panels, which is not built for the tests.
const testPanelsView = "<html></html>"

// withPanelsView makes New build the server with the given view of perses_render_panels (none when empty).
func withPanelsView(t *testing.T, view string) {
	t.Helper()
	previous := loadPanelsView
	loadPanelsView = func() (string, bool) { return view, view != "" }
	t.Cleanup(func() { loadPanelsView = previous })
}

func allTestTools(t *testing.T) []*tools.Tool {
	t.Helper()
	client := apiclient.NewWithClient(&perseshttp.RESTClient{BaseURL: common.MustParseURL("http://127.0.0.1:1")})
	return buildTools(client, testPanelsView)
}

func toolNames(selected []*tools.Tool) []string {
	var result []string
	for _, tool := range selected {
		result = append(result, tool.MCPTool.Name)
	}
	return result
}

// checkMentionedTools fails when the instructions mention a tool which is not registered.
func checkMentionedTools(t *testing.T, instructions string, registered []string) {
	t.Helper()
	for _, match := range toolNameRegexp.FindAllStringSubmatch(instructions, -1) {
		if !slices.Contains(registered, match[1]) {
			t.Errorf("instructions mention the unregistered tool %s", match[1])
		}
	}
}

// TestBuildInstructions compares the instructions with the golden files of testdata/instructions.
// Run `go test ./internal/permcp -run TestBuildInstructions -update` to update them.
func TestBuildInstructions(t *testing.T) {
	all := allTestTools(t)
	tests := []struct {
		name string
		cfg  Config
	}{
		{name: "all", cfg: Config{}},
		{name: "read-only", cfg: Config{ReadOnly: true}},
		{name: "prometheus", cfg: Config{AllowedResources: []string{"prometheus"}}},
		{name: "prometheus-globaldatasource", cfg: Config{AllowedResources: []string{"prometheus", "globaldatasource"}}},
		{name: "dashboard", cfg: Config{AllowedResources: []string{"dashboard"}}},
		{name: "prometheus-datasource", cfg: Config{AllowedResources: []string{"prometheus", "datasource"}}},
		{name: "read-only-dashboard-prometheus", cfg: Config{ReadOnly: true, AllowedResources: []string{"dashboard", "prometheus"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			selected := selectTools(all, tt.cfg).selected
			instructions := buildInstructions(selected)

			golden := filepath.Clean(filepath.Join("testdata", "instructions", tt.name+".md"))
			if *updateGolden {
				if err := os.WriteFile(golden, []byte(instructions), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			want, err := os.ReadFile(golden)
			if err != nil {
				t.Fatalf("reading the golden file (run with -update to create it): %v", err)
			}
			if instructions != string(want) {
				t.Errorf("instructions differ from %s (run with -update to update it):\n%s", golden, instructions)
			}

			checkMentionedTools(t, instructions, toolNames(selected))
			if len(instructions) > 2000 {
				t.Errorf("instructions are %d chars long", len(instructions))
			}
		})
	}
}
