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
	"testing"

	v1 "github.com/perses/perses/pkg/model/api/v1"
	"github.com/perses/spec/go/dashboard"
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
	d, err := DecodeDashboard(json.RawMessage(raw), "proj")
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestDecodeDashboard(t *testing.T) {
	d := mustDecode(t, sampleDashboard)
	equal(t, v1.KindDashboard, d.Kind)
	equal(t, "proj", d.Metadata.Project)

	d, err := DecodeDashboard(json.RawMessage(`{"metadata": {"name": "d", "project": "other"}, "spec": {"duration": "1h"}}`), "proj")
	if err != nil {
		t.Fatal(err)
	}
	equal(t, v1.KindDashboard, d.Kind)
	equal(t, "other", d.Metadata.Project)

	_, err = DecodeDashboard(json.RawMessage(`{"kind": "Datasource", "metadata": {"name": "d"}, "spec": {"duration": "1h"}}`), "proj")
	if err == nil || err.Error() != `invalid kind: "Datasource" for a Dashboard type` {
		t.Errorf("expected error %q, got %v", `invalid kind: "Datasource" for a Dashboard type`, err)
	}
}

func TestPlacePanel(t *testing.T) {
	var layouts []dashboard.Layout
	index := EnsureLayout(&layouts, "Row")
	if !equal(t, 0, index) {
		t.FailNow()
	}
	equal(t, "Row", layoutTitle(layouts[0]))

	positions := [][2]int{}
	for _, key := range []string{"a", "b", "c"} {
		item, err := PlacePanel(&layouts[index], key, 12, 6)
		if err != nil {
			t.Fatal(err)
		}
		positions = append(positions, [2]int{item.X, item.Y})
	}
	equal(t, [][2]int{{0, 0}, {12, 0}, {0, 6}}, positions)

	item, err := PlacePanel(&layouts[index], "wide", 48, 0)
	if err != nil {
		t.Fatal(err)
	}
	equal(t, GridColumns, item.Width)
	equal(t, DefaultPanelHeight, item.Height)
	equal(t, 12, item.Y)

	equal(t, 0, EnsureLayout(&layouts, "row"))
	equal(t, 0, FindLayout(layouts, ""))
	equal(t, 1, EnsureLayout(&layouts, "Other"))
	equal(t, 1, RemovePanelItems(layouts, "b"))
	hasLen(t, layouts[0].Spec.(*dashboard.GridLayoutSpec).Items, 3)
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
	if !hasLen(t, locations, 1) {
		t.FailNow()
	}
	equal(t, 0, locations[0].Layout)
	equal(t, -1, locations[0].Tab)
	locations = FindPanelItems(layouts, "b")
	if !hasLen(t, locations, 1) {
		t.FailNow()
	}
	equal(t, PanelLocation{Layout: 1, Tab: 1, Item: layouts[1].Spec.(*dashboard.TabLayoutSpec).Tabs[1].Items[0]}, locations[0])
	equal(t, `layout 1 tab 1 "second"`, DescribePanelLocation(layouts, locations[0]))
	empty(t, FindPanelItems(layouts, "missing"))

	item, err := PlacePanelInTab(&layouts[1], 0, "c", 4, 2, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	equal(t, dashboard.GridItem{X: 4, Y: 2, Width: DefaultPanelWidth, Height: DefaultPanelHeight, Content: PanelRef("c")}, item)
	if !IsPlaced(layouts, "c") {
		t.Errorf("expected true")
	}
	locations = FindPanelItems(layouts, "c")
	if !hasLen(t, locations, 1) {
		t.FailNow()
	}
	equal(t, 0, locations[0].Tab)

	_, err = PlacePanelInTab(&layouts[1], 2, "d", 0, 0, 0, 0)
	errorContains(t, err, "tab 2 does not exist")
	_, err = PlacePanelInTab(&layouts[0], 0, "d", 0, 0, 0, 0)
	errorContains(t, err, "is not a tabs layout")
	_, err = PlacePanelAt(&layouts[1], "d", 0, 0, 0, 0)
	errorContains(t, err, "is not a grid")
	if IsPlaced(layouts, "d") {
		t.Errorf("expected false")
	}
}

func TestSummarize(t *testing.T) {
	d := mustDecode(t, sampleDashboard)
	base, _ := url.Parse("http://perses.example/")
	summary := Summarize(d, base)
	equal(t, "http://perses.example/projects/proj/dashboards/sample", summary.URL)
	equal(t, 2, summary.Panels)

	outline := OutlineOf(d, base)
	if !hasLen(t, outline.PanelList, 2) {
		t.FailNow()
	}
	equal(t, []string{"layout 0: x=0 y=0 w=12 h=6"}, outline.PanelList[0].Placement)
	empty(t, outline.PanelList[1].Placement)
	equal(t, "inst", outline.VariableList[0].Name)
	equal(t, "Row", outline.LayoutList[0].Title)
}

func TestDashboardURLDropsCredentials(t *testing.T) {
	base, err := url.Parse("https://user:secret@perses.example/perses/")
	if err != nil {
		t.Fatal(err)
	}
	equal(t, "https://perses.example/perses/projects/proj/dashboards/sample", DashboardURL(base, "proj", "sample"))
	equal(t, "https://user:secret@perses.example/perses/", base.String())
}
