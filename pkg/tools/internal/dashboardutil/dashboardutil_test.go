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

package dashboardutil

import (
	"encoding/json"
	"net/url"
	"strconv"
	"testing"

	v1 "github.com/perses/perses/pkg/model/api/v1"
	"github.com/perses/spec/go/dashboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const sampleDashboard = `{
  "kind": "Dashboard",
  "metadata": {"name": "sample"},
  "spec": {
    "duration": "1h",
    "variables": [
      {"kind": "ListVariable", "spec": {"name": "inst", "plugin": {"kind": "PrometheusLabelValuesVariable", "spec": {"labelName": "instance"}}}}
    ],
    "panels": {
      "p1": {"kind": "Panel", "spec": {"display": {"name": "P1"}, "plugin": {"kind": "TimeSeriesChart", "spec": {}},
        "queries": [{"kind": "TimeSeriesQuery", "spec": {"plugin": {"kind": "PrometheusTimeSeriesQuery", "spec": {"query": "up"}}}}]}},
      "p2": {"kind": "Panel", "spec": {"plugin": {"kind": "StatChart", "spec": {"calculation": "last"}}}}
    },
    "layouts": [
      {"kind": "Grid", "spec": {"display": {"title": "Row"}, "items": [
        {"x": 0, "y": 0, "width": 12, "height": 6, "content": {"$ref": "#/spec/panels/p1"}}
      ]}}
    ]
  }
}`

func mustDecode(t *testing.T, raw string) *v1.Dashboard {
	t.Helper()
	d, issues := Decode(json.RawMessage(raw), "proj")
	require.Empty(t, issues)
	require.NotNil(t, d)
	return d
}

func TestDecode(t *testing.T) {
	t.Run("object with defaults", func(t *testing.T) {
		d := mustDecode(t, sampleDashboard)
		assert.Equal(t, "proj", d.Metadata.Project)
		assert.Equal(t, v1.KindDashboard, d.Kind)
		assert.Len(t, d.Spec.Panels, 2)
	})
	t.Run("JSON in a string", func(t *testing.T) {
		quoted := strconv.Quote(sampleDashboard)
		d := mustDecode(t, quoted)
		assert.Equal(t, "sample", d.Metadata.Name)
	})
	t.Run("syntax error", func(t *testing.T) {
		d, issues := Decode(json.RawMessage(`{"kind": "Dashboard",`), "proj")
		assert.Nil(t, d)
		require.Len(t, issues, 1)
		assert.Empty(t, issues[0].Path)
		assert.Contains(t, issues[0].Message, "malformed JSON")
	})
	t.Run("project mismatch and missing name", func(t *testing.T) {
		d, issues := Decode(json.RawMessage(`{"metadata": {"project": "other"}, "spec": {"panels": {}, "layouts": []}}`), "proj")
		assert.Nil(t, d)
		paths := []string{}
		for _, issue := range issues {
			paths = append(paths, issue.Path)
		}
		assert.ElementsMatch(t, []string{"metadata.name", "metadata.project"}, paths)
	})
	t.Run("dangling reference", func(t *testing.T) {
		raw := `{"metadata": {"name": "d"}, "spec": {"panels": {}, "layouts": [{"kind": "Grid", "spec": {"items": [
		  {"x": 0, "y": 0, "width": 12, "height": 6, "content": {"$ref": "#/spec/panels/nope"}}]}}]}}`
		d, issues := Decode(json.RawMessage(raw), "proj")
		assert.Nil(t, d)
		require.Len(t, issues, 1)
		assert.Equal(t, "spec.layouts[0].spec.items[0].content.$ref", issues[0].Path)
		assert.Contains(t, issues[0].Message, `no panel found for ref "nope"`)
	})
	t.Run("wrong type", func(t *testing.T) {
		raw := `{"metadata": {"name": "d"}, "spec": {"duration": 12, "panels": {}, "layouts": []}}`
		d, issues := Decode(json.RawMessage(raw), "proj")
		assert.Nil(t, d)
		require.NotEmpty(t, issues)
	})
}

func TestLint(t *testing.T) {
	d := mustDecode(t, sampleDashboard)
	errs, warnings := Lint(d)
	assert.Empty(t, errs)
	require.Len(t, warnings, 1)
	assert.Equal(t, "spec.panels.p2", warnings[0].Path)

	// several orphans are reported as a single warning
	d.Spec.Panels["p3"] = d.Spec.Panels["p2"]
	_, warnings = Lint(d)
	require.Len(t, warnings, 1)
	assert.Equal(t, "spec.panels", warnings[0].Path)
	assert.Contains(t, warnings[0].Message, "2 panels are not displayed: no layout item references p2, p3")
	delete(d.Spec.Panels, "p3")

	grid := d.Spec.Layouts[0].Spec.(*dashboard.GridLayoutSpec)
	grid.Items = append(grid.Items,
		dashboard.GridItem{X: 20, Y: 0, Width: 12, Height: 6, Content: PanelRef("p2")},
		dashboard.GridItem{X: 0, Y: 0, Width: 6, Height: 2, Content: PanelRef("p2")},
	)
	d.Spec.Variables = append(d.Spec.Variables, d.Spec.Variables[0])
	errs, warnings = Lint(d)
	paths := []string{}
	for _, issue := range errs {
		paths = append(paths, issue.Path)
	}
	assert.ElementsMatch(t, []string{"spec.layouts[0].spec.items[1].x", "spec.variables[1].spec.name"}, paths)
	require.Len(t, warnings, 1)
	assert.Equal(t, "spec.layouts[0].spec.items[2]", warnings[0].Path)
	assert.Contains(t, warnings[0].Message, "overlaps spec.layouts[0].spec.items[0]")
}

func TestIssuesFromServerMessage(t *testing.T) {
	d := mustDecode(t, sampleDashboard)
	testSuites := []struct {
		title    string
		message  string
		expected []Issue
	}{
		{
			title:   "panel plugin",
			message: "bad request: bad request: invalid panel p1: spec.visual.lineWidth: conflicting values \"thick\" and number (mismatched types string and number):\n    ../etc/perses/plugins/TimeSeriesChart-0.14.0/schemas/time-series.cue:26:18\n    1:1\n",
			expected: []Issue{{
				Path:    "spec.panels.p1.spec.plugin.spec.visual.lineWidth",
				Message: `conflicting values "thick" and number (mismatched types string and number)`,
			}},
		},
		{
			title:   "variable",
			message: "bad request: bad request: invalid variable inst: spec.labelNam: field not allowed:\n    1:49\n",
			expected: []Issue{{
				Path:    "spec.variables[0].spec.plugin.spec.labelNam",
				Message: "field not allowed",
				Hint:    "remove the field or fix its name; use perses_get_plugin_schema with the variable plugin kind to see the allowed fields",
			}},
		},
		{
			title:   "query",
			message: "bad request: bad request: panel \"p1\": invalid query n°1: spec.quer: field not allowed:\n    1:45\n",
			expected: []Issue{{
				Path:    "spec.panels.p1.spec.queries[0].spec.plugin.spec.quer",
				Message: "field not allowed",
				Hint:    "remove the field or fix its name; use perses_get_plugin_schema with the query plugin kind to see the allowed fields",
			}},
		},
		{
			title:   "unknown plugin",
			message: "bad request: bad request: schema not found for plugin NoSuchChart with version unknown",
			expected: []Issue{{
				Path:    "",
				Message: `the plugin "NoSuchChart" is not installed on the Perses server`,
				Hint:    "use perses_list_plugin_schemas to list the available plugins; plugin kinds are case-sensitive",
			}},
		},
		{
			title:   "echo wrapped",
			message: `bad request: code=400, message="Bad Name" is not a correct name. It should match the regexp: ^[a-zA-Z0-9_.-]+$, internal="Bad Name" is not a correct name. It should match the regexp: ^[a-zA-Z0-9_.-]+$`,
			expected: []Issue{{
				Path:    "metadata.name",
				Message: `"Bad Name" is not a correct name. It should match the regexp: ^[a-zA-Z0-9_.-]+$`,
			}},
		},
		{
			title: "two panels joined",
			message: "bad request: bad request: invalid panel p1: spec.visual.lineWidth: conflicting values \"thick\" and number:\n    1:1\n" +
				"invalid panel p2: spec.calculation: conflicting values \"mean\" and \"last\":\n    1:1\n",
			expected: []Issue{
				{Path: "spec.panels.p1.spec.plugin.spec.visual.lineWidth", Message: `conflicting values "thick" and number`},
				{Path: "spec.panels.p2.spec.plugin.spec.calculation", Message: `conflicting values "mean" and "last"`},
			},
		},
	}
	for _, test := range testSuites {
		t.Run(test.title, func(t *testing.T) {
			issues := IssuesFromServerMessage(test.message, d)
			require.Len(t, issues, len(test.expected))
			for i, expected := range test.expected {
				assert.Equal(t, expected.Path, issues[i].Path)
				assert.Equal(t, expected.Message, issues[i].Message)
				if expected.Hint != "" {
					assert.Equal(t, expected.Hint, issues[i].Hint)
				}
			}
		})
	}
}

func TestPlacePanel(t *testing.T) {
	var layouts []dashboard.Layout
	index := EnsureLayout(&layouts, "Row")
	require.Equal(t, 0, index)
	assert.Equal(t, "Row", layoutTitle(layouts[0]))

	positions := [][2]int{}
	for _, key := range []string{"a", "b", "c"} {
		item, err := PlacePanel(&layouts[index], key, 12, 6)
		require.NoError(t, err)
		positions = append(positions, [2]int{item.X, item.Y})
	}
	assert.Equal(t, [][2]int{{0, 0}, {12, 0}, {0, 6}}, positions)

	item, err := PlacePanel(&layouts[index], "wide", 48, 0)
	require.NoError(t, err)
	assert.Equal(t, GridColumns, item.Width)
	assert.Equal(t, DefaultPanelHeight, item.Height)
	assert.Equal(t, 12, item.Y)

	assert.Equal(t, 0, EnsureLayout(&layouts, "row"))
	assert.Equal(t, 0, FindLayout(layouts, ""))
	assert.Equal(t, 1, EnsureLayout(&layouts, "Other"))
	assert.Equal(t, 1, RemovePanelItems(layouts, "b"))
	assert.Len(t, layouts[0].Spec.(*dashboard.GridLayoutSpec).Items, 3)
}

func TestSummarize(t *testing.T) {
	d := mustDecode(t, sampleDashboard)
	base, _ := url.Parse("http://perses.example/")
	summary := Summarize(d, base)
	assert.Equal(t, "http://perses.example/projects/proj/dashboards/sample", summary.URL)
	assert.Equal(t, 2, summary.Panels)

	outline := OutlineOf(d, base)
	require.Len(t, outline.PanelList, 2)
	assert.Equal(t, []string{"layout 0: x=0 y=0 w=12 h=6"}, outline.PanelList[0].Placement)
	assert.Empty(t, outline.PanelList[1].Placement)
	assert.Equal(t, "inst", outline.VariableList[0].Name)
	assert.Equal(t, "Row", outline.LayoutList[0].Title)
}
