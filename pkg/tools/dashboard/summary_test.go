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
	"encoding/json"
	"net/url"
	"strings"
	"testing"

	v1 "github.com/perses/perses/pkg/model/api/v1"
	dashboardSpec "github.com/perses/spec/go/dashboard"
)

const sampleDashboard = `{
  "kind": "Dashboard",
  "metadata": {"name": "sample", "project": "proj"},
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

func TestSummarize(t *testing.T) {
	d := &v1.Dashboard{}
	if err := json.Unmarshal([]byte(sampleDashboard), d); err != nil {
		t.Fatal(err)
	}
	base, _ := url.Parse("http://perses.example/")
	summary := summarize(d, "proj", base)
	equal(t, "http://perses.example/projects/proj/dashboards/sample", summary.URL)
	equal(t, 2, summary.Panels)

	outline := outlineOf(d, base)
	if !hasLen(t, outline.PanelList, 2) {
		t.FailNow()
	}
	equal(t, []string{"layout 0: x=0 y=0 w=12 h=6"}, outline.PanelList[0].Placement)
	empty(t, outline.PanelList[1].Placement)
	equal(t, "inst", outline.VariableList[0].Name)
	equal(t, "Row", outline.LayoutList[0].Title)
}

func TestPanelRefWithoutHash(t *testing.T) {
	d := &v1.Dashboard{}
	if err := json.Unmarshal([]byte(strings.Replace(sampleDashboard, `"#/spec/panels/p1"`, `"/spec/panels/p1"`, 1)), d); err != nil {
		t.Fatal(err)
	}
	equal(t, "p1", panelKeyOf(d.Spec.Layouts[0].Spec.(*dashboardSpec.GridLayoutSpec).Items[0]))
	outline := outlineOf(d, nil)
	equal(t, []string{"layout 0: x=0 y=0 w=12 h=6"}, outline.PanelList[0].Placement)
}

func TestDashboardURLDropsCredentials(t *testing.T) {
	base, err := url.Parse("https://user:secret@perses.example/perses/")
	if err != nil {
		t.Fatal(err)
	}
	equal(t, "https://perses.example/perses/projects/proj/dashboards/sample", dashboardURL(base, "proj", "sample"))
	equal(t, "https://user:secret@perses.example/perses/", base.String())
}

func TestDashboardURLEscapesNamesOnce(t *testing.T) {
	base, err := url.Parse("https://perses.example/perses?tab=1#top")
	if err != nil {
		t.Fatal(err)
	}
	equal(t, "https://perses.example/perses/projects/my%20proj/dashboards/a%20b", dashboardURL(base, "my proj", "a b"))
}
