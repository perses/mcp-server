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

package dashboard

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// GuideURI is the URI of the MCP resource describing the Perses dashboard model.
const GuideURI = "perses://guides/dashboard-model"

// persesModel is the minimum a model must know to produce a dashboard that renders. It is deliberately short: the
// details live in the GuideURI resource, which clients can attach on demand.
const persesModel = "Model: spec.panels is a map whose keys are referenced as '#/spec/panels/<key>'; a panel is " +
	"displayed only if a spec.layouts item references it (24-column grid, x/y/width/height). Omit spec.datasource to " +
	"use the default datasource. Full guide with an example: read the MCP resource " + GuideURI + "; plugin specs: " +
	"perses_list_plugin_schemas / perses_get_plugin_schema."

const guide = `# Perses dashboard model

A dashboard is a JSON document:

` + "```json" + `
{
  "kind": "Dashboard",
  "metadata": {"name": "api", "project": "<project>"},
  "spec": {
    "display": {"name": "API"},
    "duration": "1h",
    "variables": [
      {"kind": "ListVariable", "spec": {"name": "job", "allowAllValue": true,
        "plugin": {"kind": "PrometheusLabelValuesVariable", "spec": {"labelName": "job"}}}}
    ],
    "panels": {
      "rps": {
        "kind": "Panel",
        "spec": {
          "display": {"name": "Requests/s"},
          "plugin": {"kind": "TimeSeriesChart", "spec": {}},
          "queries": [{"kind": "TimeSeriesQuery", "spec": {"plugin": {
            "kind": "PrometheusTimeSeriesQuery", "spec": {"query": "sum(rate(http_requests_total{job=~\"$job\"}[5m]))"}}}}]
        }
      }
    },
    "layouts": [
      {"kind": "Grid", "spec": {"display": {"title": "Traffic"}, "items": [
        {"x": 0, "y": 0, "width": 12, "height": 6, "content": {"$ref": "#/spec/panels/rps"}}
      ]}}
    ]
  }
}
` + "```" + `

## Rules the server does not enforce

- ` + "`spec.panels`" + ` is a **map**, not an array. The key (` + "`[a-zA-Z0-9_.-]+`" + `) is the panel identity and is
  referenced as ` + "`#/spec/panels/<key>`" + `.
- A panel is **rendered only if a layout item references it**. Unreferenced panels are saved but invisible.
- Layouts are rows (` + "`kind: Grid`" + `, optional ` + "`display.title`" + ` and ` + "`display.collapse.open`" + `). Items are placed on a
  **24-column grid**: ` + "`x + width <= 24`" + `, a row unit is 30px, a chart is usually 12x6 (half width) or 24x6 (full width),
  a stat is usually 6x3 or 4x3. Overlapping items are a mistake.
- Variable names must be unique and are used in queries as ` + "`$name`" + `.
- Omit ` + "`spec.datasource`" + ` in a query to use the default datasource of the project / server. Set
  ` + "`{\"kind\": \"PrometheusDatasource\", \"name\": \"<datasource>\"}`" + ` only to target a specific one.
- Plugin kinds are case-sensitive (` + "`TimeSeriesChart`" + `, ` + "`StatChart`" + `, ` + "`GaugeChart`" + `, ` + "`Table`" + `,
  ` + "`BarChart`" + `, ` + "`PieChart`" + `, ` + "`Markdown`" + `, ` + "`PrometheusTimeSeriesQuery`" + `, ...). Use
  ` + "`perses_list_plugin_schemas`" + ` to list what is installed and ` + "`perses_get_plugin_schema`" + ` for the fields of a spec.

## Efficient workflow with the tools

1. ` + "`perses_create_dashboard`" + ` with a skeleton (display, duration, variables, an empty ` + "`panels`" + ` map and
   ` + "`layouts`" + ` list). Pass the dashboard as a JSON object, not as an escaped string. Use ` + "`dry_run`" + ` to
   validate without saving and ` + "`if_exists: replace`" + ` to make the call idempotent.
2. ` + "`perses_set_dashboard_panels`" + ` to add panels group by group: panels are validated individually and placed
   automatically in the ` + "`layout`" + ` group (created when missing), so no layout item has to be written by hand.
   Re-sending an existing key replaces the panel and keeps its position; ` + "`placements`" + ` alone moves/resizes
   existing panels without resending definitions. Explicit positions are reserved before automatic placement.
   Use ` + "`order: [\"b\", \"a\", \"c\"]`" + ` for the desired auto-placement order; remaining keys are alphabetical,
   not JSON object order. Supply at least one panel or placement.
3. ` + "`perses_set_dashboard_variables`" + `, ` + "`perses_set_dashboard_layout`" + `, ` + "`perses_remove_dashboard_panels`" + `
   for the remaining adjustments; every call returns a short summary and the dashboard URL.
4. ` + "`perses_get_dashboard_by_name`" + ` with ` + "`summary: true`" + ` to review the outline instead of the full document.

For every update or incremental edit, pass ` + "`expected_version`" + ` from your most recent dashboard read. If the
version is stale, re-read and reapply the intended change. This is a best-effort check, not an atomic lock: Perses
does not provide conditional writes, so a concurrent update after the check can still race.

Validation errors are returned as ` + "`{\"valid\": false, \"errors\": [{\"path\", \"message\", \"hint\"}]}`" + ` where
` + "`path`" + ` is the dashboard field to fix (for example ` + "`spec.panels.rps.spec.plugin.spec.visual.lineWidth`" + `).
`

// GuideResource returns the MCP resource holding the dashboard guide and its handler.
func GuideResource() (*mcp.Resource, mcp.ResourceHandler) {
	res := &mcp.Resource{
		URI:         GuideURI,
		Name:        "perses-dashboard-model",
		Title:       "Perses dashboard model",
		Description: "How a Perses dashboard is structured (panels map, layout items on a 24-column grid, variables, default datasource) and how to build one incrementally with the dashboard tools.",
		MIMEType:    "text/markdown",
	}
	handler := func(_ context.Context, _ *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		return &mcp.ReadResourceResult{
			Contents: []*mcp.ResourceContents{{URI: GuideURI, MIMEType: "text/markdown", Text: guide}},
		}, nil
	}
	return res, handler
}
