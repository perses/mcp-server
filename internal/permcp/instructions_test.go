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
	"flag"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	apiclient "github.com/perses/perses/pkg/client/api"
	"github.com/perses/perses/pkg/client/config"
	"github.com/perses/perses/pkg/client/perseshttp"
	"github.com/perses/spec/go/common"

	"github.com/perses/mcp-server/pkg/tools"
	"github.com/perses/mcp-server/pkg/tools/prometheus"
)

var updateGolden = flag.Bool("update", false, "update the golden files of the instructions")

// toolNameRegexp matches the tool names of the instructions: buildInstructions always formats them as code.
var toolNameRegexp = regexp.MustCompile("`(perses_[a-z_]+)`")

func allTestTools(t *testing.T) []*tools.Tool {
	t.Helper()
	client := apiclient.NewWithClient(&perseshttp.RESTClient{BaseURL: common.MustParseURL("http://127.0.0.1:1")})
	return buildTools(client)
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

// The instructions silently skip the tools which are not registered: a renamed tool would disappear from them.
func TestInstructionToolNamesExist(t *testing.T) {
	all := toolNames(allTestTools(t))
	for _, name := range []string{
		listGlobalDatasourcesTool, listProjectDatasourcesTool, queryGlobalDatasourceTool, queryProjectDatasourceTool,
		listDashboardsTool, getDashboardTool, prometheus.ListMetricNamesToolName, prometheus.ListLabelNamesToolName,
		prometheus.ListLabelValuesToolName, prometheus.GetMetricMetadataToolName, prometheus.QueryToolName,
	} {
		if !slices.Contains(all, name) {
			t.Errorf("the tool %s used by the instructions does not exist", name)
		}
	}
}

func TestNewSendsTheInstructions(t *testing.T) {
	cfg := Config{
		ReadOnly:         true,
		AllowedResources: []string{"dashboard", "prometheus"},
		PersesServer:     config.RestConfigClient{URL: common.MustParseURL("http://127.0.0.1:1")},
	}
	task, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := task.(*server).mcpServer.Connect(context.Background(), serverTransport, nil)
	if err != nil {
		t.Fatalf("connecting the server: %v", err)
	}
	t.Cleanup(func() { _ = serverSession.Close() })
	session, err := mcp.NewClient(&mcp.Implementation{Name: "test-client"}, nil).Connect(context.Background(), clientTransport, nil)
	if err != nil {
		t.Fatalf("connecting the client: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })

	toolList, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	var registered []string
	for _, tool := range toolList.Tools {
		registered = append(registered, tool.Name)
	}
	want := toolNames(selectTools(allTestTools(t), cfg).selected)
	slices.Sort(registered)
	slices.Sort(want)
	if !slices.Equal(registered, want) {
		t.Errorf("registered tools = %v, want %v", registered, want)
	}
	if slices.Contains(registered, "perses_create_dashboard") {
		t.Error("a write tool is registered in read-only mode")
	}

	instructions := session.InitializeResult().Instructions
	if instructions != buildInstructions(selectTools(allTestTools(t), cfg).selected) {
		t.Errorf("instructions =\n%s", instructions)
	}
	checkMentionedTools(t, instructions, registered)
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
