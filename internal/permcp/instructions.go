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
	"fmt"
	"strings"

	"github.com/perses/common/set"

	"github.com/perses/mcp-server/pkg/tools"
	"github.com/perses/mcp-server/pkg/tools/dashboard"
	"github.com/perses/mcp-server/pkg/tools/datasource"
	"github.com/perses/mcp-server/pkg/tools/globaldatasource"
	"github.com/perses/mcp-server/pkg/tools/prometheus"
)

// buildInstructions returns the server instructions sent to every client on initialize: what Perses is and how to
// answer questions about monitored systems. They only mention the registered tools. They describe the order and the
// choice of the tools: what a tool does and its parameters are in its description.
func buildInstructions(registered []*tools.Tool) string {
	names := set.New[string]()
	for _, tool := range registered {
		names.Add(tool.MCPTool.Name)
	}
	// code returns the registered tools among toolNames, formatted as code.
	code := func(toolNames ...string) []string {
		var available []string
		for _, name := range toolNames {
			if names.Contains(name) {
				available = append(available, "`"+name+"`")
			}
		}
		return available
	}

	var b strings.Builder
	b.WriteString("Perses is an open-source observability dashboard tool. " +
		"This server gives access to Perses resources.\n")

	// The Prometheus tools are all read-only tools of the same resource: they are all registered or none is.
	if !names.Contains(prometheus.QueryToolName) {
		return b.String()
	}

	datasourceStep := "Datasource: use the same `datasource` for discovery and queries"
	if listTools := code(globaldatasource.ListToolName, datasource.ListToolName); len(listTools) > 0 {
		datasourceStep += "; find the other datasources with " + strings.Join(listTools, " and ")
	}
	datasourceStep += "."

	queryStep := fmt.Sprintf("Query with `%s`", prometheus.QueryToolName)
	if proxyTools := code(globaldatasource.QueryToolName, datasource.QueryToolName); len(proxyTools) > 0 {
		queryStep += ", not the raw datasource proxy " + strings.Join(proxyTools, " or ")
	}
	queryStep += fmt.Sprintf(". For a past time range, pass the same `time_range` to `%s`, `%s` and `%s`.",
		prometheus.ListMetricNamesToolName, prometheus.ListLabelNamesToolName, prometheus.ListLabelValuesToolName)

	steps := []string{
		datasourceStep,
		fmt.Sprintf("Before writing PromQL, check the metric and label names with `%s`, `%s` and `%s`, and the metric "+
			"types with `%s`.", prometheus.ListMetricNamesToolName, prometheus.ListLabelNamesToolName,
			prometheus.ListLabelValuesToolName, prometheus.GetMetricMetadataToolName),
		queryStep,
	}
	if names.Contains(prometheus.RenderPanelsToolName) {
		steps = append(steps, fmt.Sprintf("When a chart or a table helps the user, show the data with `%s`: it "+
			"returns statistics of the series, and clients that support MCP Apps display the panels.",
			prometheus.RenderPanelsToolName))
	}
	b.WriteString("\nTo answer questions about monitored systems (metrics):\n")
	for i, step := range steps {
		fmt.Fprintf(&b, "%d. %s\n", i+1, step)
	}

	if dashboards := code(dashboard.ListToolName, dashboard.GetToolName); len(dashboards) > 0 {
		fmt.Fprintf(&b, "\nThe panels of existing dashboards (%s) show which metrics and queries matter for a system.\n",
			strings.Join(dashboards, ", "))
	}
	return b.String()
}
