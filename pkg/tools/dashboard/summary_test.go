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
	"testing"

	v1 "github.com/perses/perses/pkg/model/api/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	require.NoError(t, json.Unmarshal([]byte(sampleDashboard), d))
	base, _ := url.Parse("http://perses.example/")
	summary := Summarize(d, "proj", base)
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
