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
	t.Run("null panels and datasources", func(t *testing.T) {
		raw := `{"metadata": {"name": "d"}, "spec": {"panels": {"p1": null, "cpu.load": null}, "datasources": {"prom": null},
		  "layouts": [{"kind": "Grid", "spec": {"items": [{"x": 0, "y": 0, "width": 12, "height": 6, "content": {"$ref": "#/spec/panels/p1"}}]}}]}}`
		d, issues := Decode(json.RawMessage(raw), "proj")
		assert.Nil(t, d)
		paths := []string{}
		for _, issue := range issues {
			paths = append(paths, issue.Path)
			assert.Equal(t, "the value cannot be null", issue.Message)
		}
		assert.Equal(t, []string{`spec.panels["cpu.load"]`, "spec.panels.p1", "spec.datasources.prom"}, paths)
	})
	t.Run("duplicate variable", func(t *testing.T) {
		raw := `{"metadata": {"name": "d"}, "spec": {"panels": {}, "layouts": [], "variables": [
		  {"kind": "TextVariable", "spec": {"name": "inst", "value": "a"}},
		  {"kind": "TextVariable", "spec": {"name": "inst", "value": "b"}}]}}`
		d, issues := Decode(json.RawMessage(raw), "proj")
		assert.Nil(t, d)
		require.Len(t, issues, 1)
		assert.Equal(t, "spec.variables[1].spec.name", issues[0].Path)
		assert.Equal(t, `variable "inst" (index 1) already exists`, issues[0].Message)
	})
	t.Run("wrong type", func(t *testing.T) {
		raw := `{"metadata": {"name": "d"}, "spec": {"duration": 12, "panels": {}, "layouts": []}}`
		d, issues := Decode(json.RawMessage(raw), "proj")
		assert.Nil(t, d)
		require.NotEmpty(t, issues)
	})
}

func TestDecodeMissingPlugin(t *testing.T) {
	dashboardWith := func(panel string) string {
		return `{"metadata": {"name": "d"}, "spec": {"panels": {"cpu": ` + panel + `}, "layouts": []}}`
	}
	panelWithQuery := func(query string) string {
		return dashboardWith(`{"kind": "Panel", "spec": {"plugin": {"kind": "TimeSeriesChart", "spec": {}}, "queries": [` + query + `]}}`)
	}
	tests := []struct {
		name    string
		raw     string
		path    string
		message string
		hint    string
	}{
		{
			name:    "plugin given as the query",
			raw:     panelWithQuery(`{"kind": "PrometheusTimeSeriesQuery", "spec": {"query": "up"}}`),
			path:    "spec.panels.cpu.spec.queries[0]",
			message: `query plugin is missing: "PrometheusTimeSeriesQuery" is not a query kind`,
			hint:    `queries need a wrapper: {kind: "TimeSeriesQuery", spec: {plugin: {kind: "PrometheusTimeSeriesQuery", spec: {query: ...}}}}`,
		},
		{
			name:    "unknown query kind",
			raw:     panelWithQuery(`{"kind": "PrometheusQuery", "spec": {"query": "up"}}`),
			path:    "spec.panels.cpu.spec.queries[0]",
			message: `query plugin is missing: "PrometheusQuery" is not a query kind`,
			hint:    `queries need a wrapper: {kind: "TimeSeriesQuery", spec: {plugin: {kind: "PrometheusQuery", spec: {query: ...}}}}`,
		},
		{
			name:    "trace plugin given as the query",
			raw:     panelWithQuery(`{"kind": "TempoTraceQuery", "spec": {}}`),
			path:    "spec.panels.cpu.spec.queries[0]",
			message: `query plugin is missing: "TempoTraceQuery" is not a query kind`,
			hint:    `queries need a wrapper: {kind: "TraceQuery", spec: {plugin: {kind: "TempoTraceQuery", spec: {...}}}}`,
		},
		{
			name:    "query without spec",
			raw:     panelWithQuery(`{"kind": "TimeSeriesQuery"}`),
			path:    "spec.panels.cpu.spec.queries[0]",
			message: "query plugin is missing",
			hint:    `queries need a wrapper: {kind: "TimeSeriesQuery", spec: {plugin: {kind: "PrometheusTimeSeriesQuery", spec: {...}}}}`,
		},
		{
			name:    "panel without plugin",
			raw:     dashboardWith(`{"kind": "Panel", "spec": {"display": {"name": "CPU"}}}`),
			path:    "spec.panels.cpu.spec.plugin",
			message: "panel plugin is missing",
			hint:    `panels need a plugin: {kind: "TimeSeriesChart", spec: {...}}; perses_list_plugins lists the panel plugins`,
		},
		{
			name:    "panel with a dotted key",
			raw:     `{"metadata": {"name": "d"}, "spec": {"panels": {"cpu.load": {"kind": "Panel", "spec": {"display": {"name": "CPU"}}}}, "layouts": []}}`,
			path:    `spec.panels["cpu.load"].spec.plugin`,
			message: "panel plugin is missing",
			hint:    `panels need a plugin: {kind: "TimeSeriesChart", spec: {...}}; perses_list_plugins lists the panel plugins`,
		},
		{
			name: "list variable without plugin",
			raw: `{"metadata": {"name": "d"}, "spec": {"panels": {}, "layouts": [],
			  "variables": [{"kind": "TextVariable", "spec": {"name": "t", "value": "x"}}, {"kind": "ListVariable", "spec": {"name": "inst"}}]}}`,
			path:    "spec.variables[1].spec.plugin",
			message: "list variable plugin is missing",
			hint:    `list variables need a plugin: {kind: "PrometheusLabelValuesVariable", spec: {...}}`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			d, issues := Decode(json.RawMessage(test.raw), "proj")
			assert.Nil(t, d)
			require.Len(t, issues, 1)
			assert.Equal(t, Issue{Path: test.path, Message: test.message, Hint: test.hint}, issues[0])
		})
	}
	t.Run("valid dashboard has no issues", func(t *testing.T) {
		mustDecode(t, sampleDashboard)
	})
}

func TestDecodeValueMissingPlugin(t *testing.T) {
	t.Run("panels", func(t *testing.T) {
		raw := `{"cpu": {"kind": "Panel", "spec": {"plugin": {"kind": "TimeSeriesChart", "spec": {}},
		  "queries": [{"kind": "PrometheusTimeSeriesQuery", "spec": {"query": "up"}}]}}}`
		var panels map[string]*dashboard.Panel
		issue := DecodeValue(json.RawMessage(raw), "panels", &panels)
		require.NotNil(t, issue)
		assert.Equal(t, "panels.cpu.spec.queries[0]", issue.Path)
		assert.Contains(t, issue.Hint, `{kind: "TimeSeriesQuery", spec: {plugin: {kind: "PrometheusTimeSeriesQuery"`)
	})
	t.Run("variables", func(t *testing.T) {
		var variables []dashboard.Variable
		issue := DecodeValue(json.RawMessage(`[{"kind": "ListVariable", "spec": {"name": "inst"}}]`), "variables", &variables)
		require.NotNil(t, issue)
		assert.Equal(t, "variables[0].spec.plugin", issue.Path)
	})
	t.Run("valid panels", func(t *testing.T) {
		var panels map[string]*dashboard.Panel
		raw := `{"cpu": {"kind": "Panel", "spec": {"plugin": {"kind": "TimeSeriesChart", "spec": {}},
		  "queries": [{"kind": "TimeSeriesQuery", "spec": {"plugin": {"kind": "PrometheusTimeSeriesQuery", "spec": {"query": "up"}}}}]}}}`
		require.Nil(t, DecodeValue(json.RawMessage(raw), "panels", &panels))
		assert.Equal(t, "PrometheusTimeSeriesQuery", panels["cpu"].Spec.Queries[0].Spec.Plugin.Kind)
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
				Message: `the Perses server has no schema for the plugin "NoSuchChart": the plugin is not installed, or not with the requested version or registry`,
				Hint:    "check the kind of the plugin (case-sensitive) and its metadata (version, registry); use perses_list_plugin_schemas to list the available plugins",
			}},
		},
		{
			title:   "installed plugin with an unavailable version",
			message: "bad request: bad request: invalid panel p1: spec.foo: field not allowed:\n    1:1\n\nschema not found for plugin StatChart with version 9.9.9",
			expected: []Issue{
				{Path: "spec.panels.p1.spec.plugin.spec.foo", Message: "field not allowed"},
				{
					Path:    "",
					Message: `the Perses server has no schema for the plugin "StatChart" with version 9.9.9: the plugin is not installed, or not with the requested version or registry`,
				},
			},
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
		{
			title: "missing schema after an invalid panel",
			message: "bad request: bad request: invalid panel p1: spec.legend.position: 2 errors in empty disjunction:\n" +
				"spec.legend.position: conflicting values \"bottom\" and \"zz\":\n    1:1\n" +
				"spec.legend.position: conflicting values \"right\" and \"zz\":\n    1:1\n\n" +
				"schema not found for plugin NoSuchChart with version unknown\n" +
				"panel \"p1\": schema not found for plugin PrometheusTimeSeriesQuery with version unknown",
			expected: []Issue{
				{
					Path:    "spec.panels.p1.spec.plugin.spec.legend.position",
					Message: `2 errors in empty disjunction; conflicting values "bottom" and "zz"; conflicting values "right" and "zz"`,
				},
				{Path: "", Message: `the Perses server has no schema for the plugin "NoSuchChart": the plugin is not installed, or not with the requested version or registry`},
				{Path: "spec.panels.p1.spec.queries[0].spec.plugin", Message: `the Perses server has no schema for the plugin "PrometheusTimeSeriesQuery": the plugin is not installed, or not with the requested version or registry`},
			},
		},
		{
			title: "two fields of one plugin",
			message: "bad request: bad request: invalid panel p1: spec.legend.mode: 2 errors in empty disjunction:\n" +
				"spec.legend.mode: conflicting values \"list\" and \"zz\":\n    1:1\n" +
				"spec.legend.mode: conflicting values \"table\" and \"zz\":\n    1:1\n" +
				"spec.visual.lineWidth: conflicting values \"x\" and number (mismatched types string and number):\n    1:1\n" +
				"spec.foo: field not allowed:\n    1:1\n",
			expected: []Issue{
				{
					Path:    "spec.panels.p1.spec.plugin.spec.legend.mode",
					Message: `2 errors in empty disjunction; conflicting values "list" and "zz"; conflicting values "table" and "zz"`,
				},
				{Path: "spec.panels.p1.spec.plugin.spec.visual.lineWidth", Message: `conflicting values "x" and number (mismatched types string and number)`},
				{
					Path:    "spec.panels.p1.spec.plugin.spec.foo",
					Message: "field not allowed",
					Hint:    "remove the field or fix its name; use perses_get_plugin_schema with the panel plugin kind to see the allowed fields",
				},
			},
		},
		{
			title: "dotted keys",
			message: "bad request: invalid panel cpu.load: spec.foo: field not allowed:\n    1:1\n" +
				"invalid datasource my.prom: spec.bar: field not allowed:\n    1:1\n",
			expected: []Issue{
				{Path: `spec.panels["cpu.load"].spec.plugin.spec.foo`, Message: "field not allowed"},
				{Path: `spec.datasources["my.prom"].plugin.spec.bar`, Message: "field not allowed"},
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

func TestAnnotationIssuesFromServerMessage(t *testing.T) {
	d := mustDecode(t, sampleDashboard)
	annotation := func(name string) dashboard.AnnotationSpec {
		return dashboard.AnnotationSpec{Display: dashboard.AnnotationDisplay{Name: name}}
	}
	d.Spec.Annotations = []dashboard.AnnotationSpec{annotation("Deploy"), annotation("Deploy: prod"), annotation("Deploy: prod")}
	d.Spec.Panels["p1"].Spec.Annotations = []dashboard.AnnotationSpec{annotation("Restarts")}
	type expectation struct{ path, message string }
	testSuites := []struct {
		title    string
		message  string
		expected []expectation
	}{
		{
			// Messages returned by Perses for these annotations.
			title: "dashboard annotations with the same name",
			message: "bad request: bad request: invalid annotation Deploy: prod: spec.foo: field not allowed:\n    1:46\n" +
				"spec.query: field not allowed:\n    1:54\n\n" +
				"invalid annotation Deploy: prod: spec.bar: field not allowed:\n    1:46\n",
			expected: []expectation{
				{"spec.annotations[name=Deploy: prod].plugin.spec.foo", "field not allowed"},
				{"spec.annotations[name=Deploy: prod].plugin.spec.query", "field not allowed"},
				{"spec.annotations[name=Deploy: prod].plugin.spec.bar", "field not allowed"},
			},
		},
		{
			// Only the second annotation named "Deploy: prod" fails: the error cannot tell which one.
			title:    "one of the dashboard annotations with the same name",
			message:  "bad request: bad request: invalid annotation Deploy: prod: spec.bar: field not allowed:\n    1:46\n",
			expected: []expectation{{"spec.annotations[name=Deploy: prod].plugin.spec.bar", "field not allowed"}},
		},
		{
			title:    "annotation whose name is a prefix of another one",
			message:  "bad request: bad request: invalid annotation Deploy: spec.bar: field not allowed:\n    1:46\n",
			expected: []expectation{{"spec.annotations[0].plugin.spec.bar", "field not allowed"}},
		},
		{
			title:    "panel annotation",
			message:  "bad request: bad request: panel \"p1\": invalid annotation Restarts: spec.baz: field not allowed:\n    1:46\n",
			expected: []expectation{{"spec.panels.p1.spec.annotations[0].plugin.spec.baz", "field not allowed"}},
		},
		{
			title:    "unknown annotation",
			message:  "invalid annotation Other: spec.baz: field not allowed:\n    1:46\n",
			expected: []expectation{{"spec.annotations[name=Other].plugin.spec.baz", "field not allowed"}},
		},
	}
	for _, test := range testSuites {
		t.Run(test.title, func(t *testing.T) {
			issues := IssuesFromServerMessage(test.message, d)
			require.Len(t, issues, len(test.expected))
			for i, expected := range test.expected {
				assert.Equal(t, expected.path, issues[i].Path)
				assert.Equal(t, expected.message, issues[i].Message)
				assert.Contains(t, issues[i].Hint, "annotation plugin kind")
			}
		})
	}
	issues := IssuesFromServerMessage("invalid annotation Deploy: prod: spec.foo: field not allowed:\n    1:46\n", nil)
	require.Len(t, issues, 1)
	assert.Equal(t, "spec.annotations[name=Deploy: prod].plugin.spec.foo", issues[0].Path)
}

func TestMissingSchemaOfPanelQueries(t *testing.T) {
	d := mustDecode(t, `{
  "kind": "Dashboard",
  "metadata": {"name": "sample"},
  "spec": {
    "duration": "1h",
    "panels": {
      "p1": {"kind": "Panel", "spec": {"plugin": {"kind": "Markdown", "spec": {}},
        "queries": [{"kind": "TimeSeriesQuery", "spec": {"plugin": {"kind": "NoSuchQuery", "spec": {}}}}]}},
      "p2": {"kind": "Panel", "spec": {"plugin": {"kind": "Markdown", "spec": {}},
        "queries": [
          {"kind": "TimeSeriesQuery", "spec": {"plugin": {"kind": "NoSuchQuery", "metadata": {"version": "1.0.0"}, "spec": {}}}},
          {"kind": "TimeSeriesQuery", "spec": {"plugin": {"kind": "NoSuchQuery", "spec": {}}}}
        ],
        "annotations": [{"display": {"name": "Deploys"}, "plugin": {"kind": "NoSuchAnnotation", "spec": {}}}]}}
    },
    "layouts": []
  }
}`)
	issues := IssuesFromServerMessage("bad request: panel \"p2\": schema not found for plugin NoSuchQuery with version unknown\n"+
		"panel \"p2\": schema not found for plugin NoSuchQuery with version 1.0.0\n"+
		"panel \"p2\": schema not found for plugin NoSuchAnnotation with version unknown\n"+
		"panel \"p3\": schema not found for plugin NoSuchQuery with version unknown", d)
	paths := make([]string, 0, len(issues))
	for _, issue := range issues {
		paths = append(paths, issue.Path)
	}
	assert.Equal(t, []string{
		"spec.panels.p2.spec.queries[1].spec.plugin",
		"spec.panels.p2.spec.queries[0].spec.plugin",
		"spec.panels.p2.spec.annotations[0].plugin",
		"",
	}, paths)
	assert.Contains(t, issues[1].Message, `"NoSuchQuery" with version 1.0.0`)
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

func TestPanelInTab(t *testing.T) {
	layouts := []dashboard.Layout{
		{Kind: dashboard.KindGridLayout, Spec: &dashboard.GridLayoutSpec{Items: []dashboard.GridItem{{Width: 6, Height: 6, Content: PanelRef("a")}}}},
		{Kind: dashboard.KindTabLayout, Spec: &dashboard.TabLayoutSpec{Tabs: []dashboard.TabItem{
			{Name: "first"},
			{Name: "second", Items: []dashboard.GridItem{{X: 2, Width: 8, Height: 4, Content: PanelRef("b")}}},
		}}},
	}
	locations := FindPanelItems(layouts, "a")
	require.Len(t, locations, 1)
	assert.Equal(t, 0, locations[0].Layout)
	assert.Equal(t, -1, locations[0].Tab)
	locations = FindPanelItems(layouts, "b")
	require.Len(t, locations, 1)
	assert.Equal(t, PanelLocation{Layout: 1, Tab: 1, Item: layouts[1].Spec.(*dashboard.TabLayoutSpec).Tabs[1].Items[0]}, locations[0])
	assert.Equal(t, `layout 1 tab 1 "second"`, DescribePanelLocation(layouts, locations[0]))
	assert.Empty(t, FindPanelItems(layouts, "missing"))

	item, err := PlacePanelInTab(&layouts[1], 0, "c", 4, 2, 0, 0)
	require.NoError(t, err)
	assert.Equal(t, dashboard.GridItem{X: 4, Y: 2, Width: DefaultPanelWidth, Height: DefaultPanelHeight, Content: PanelRef("c")}, item)
	assert.True(t, IsPlaced(layouts, "c"))
	locations = FindPanelItems(layouts, "c")
	require.Len(t, locations, 1)
	assert.Equal(t, 0, locations[0].Tab)

	_, err = PlacePanelInTab(&layouts[1], 2, "d", 0, 0, 0, 0)
	assert.ErrorContains(t, err, "tab 2 does not exist")
	_, err = PlacePanelInTab(&layouts[0], 0, "d", 0, 0, 0, 0)
	assert.ErrorContains(t, err, "is not a tabs layout")
	_, err = PlacePanelAt(&layouts[1], "d", 0, 0, 0, 0)
	assert.ErrorContains(t, err, "is not a grid")
	assert.False(t, IsPlaced(layouts, "d"))
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

func TestDashboardURLDropsCredentials(t *testing.T) {
	base, err := url.Parse("https://user:secret@perses.example/perses/")
	require.NoError(t, err)
	assert.Equal(t, "https://perses.example/perses/projects/proj/dashboards/sample", DashboardURL(base, "proj", "sample"))
	assert.Equal(t, "https://user:secret@perses.example/perses/", base.String())
}

func TestKeyPath(t *testing.T) {
	assert.Equal(t, "spec.panels.cpu_load-1", keyPath("spec.panels", "cpu_load-1"))
	assert.Equal(t, `spec.panels["cpu.load"]`, keyPath("spec.panels", "cpu.load"))
	assert.Equal(t, `spec.panels["0cpu"]`, keyPath("spec.panels", "0cpu"))
}

func TestDecodeNonStringProject(t *testing.T) {
	d, issues := Decode(json.RawMessage(`{"metadata": {"name": "d", "project": 42}, "spec": {"panels": {}, "layouts": []}}`), "proj")
	assert.Nil(t, d)
	require.Len(t, issues, 1)
	assert.Equal(t, "metadata.project", issues[0].Path)
	assert.Equal(t, "metadata.project must be a string", issues[0].Message)
}

func TestDecodeValue(t *testing.T) {
	t.Run("numbers are preserved", func(t *testing.T) {
		var fields map[string]any
		require.Nil(t, DecodeValue(json.RawMessage(`{"a": 1e3, "b": 12345678901234567890, "c": 0.1}`), "value", &fields))
		encoded, err := json.Marshal(fields)
		require.NoError(t, err)
		assert.JSONEq(t, `{"a": 1e3, "b": 12345678901234567890, "c": 0.1}`, string(encoded))
		assert.Contains(t, string(encoded), `"a":1e3`)
		assert.Contains(t, string(encoded), `"b":12345678901234567890`)
	})
	t.Run("trailing JSON", func(t *testing.T) {
		var fields map[string]any
		issue := DecodeValue(json.RawMessage(`{"a": 1} {"b": 2}`), "value", &fields)
		require.NotNil(t, issue)
		assert.Equal(t, "value", issue.Path)
		assert.Contains(t, issue.Message, "unexpected JSON value after the value")
	})
	t.Run("type error", func(t *testing.T) {
		var target struct {
			Name string `json:"name"`
		}
		issue := DecodeValue(json.RawMessage(`{"name": 1}`), "value", &target)
		require.NotNil(t, issue)
		assert.Equal(t, "value.name", issue.Path)
	})
}
