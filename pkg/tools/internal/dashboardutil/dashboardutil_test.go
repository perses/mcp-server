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
	"testing"

	v1 "github.com/perses/perses/pkg/model/api/v1"
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
	d, err := DecodeDashboard(json.RawMessage(raw), "proj")
	require.NoError(t, err)
	return d
}

func TestDecodeDashboard(t *testing.T) {
	d := mustDecode(t, sampleDashboard)
	assert.Equal(t, v1.KindDashboard, d.Kind)
	assert.Equal(t, "proj", d.Metadata.Project)

	d, err := DecodeDashboard(json.RawMessage(`{"metadata": {"name": "d", "project": "other"}, "spec": {"duration": "1h"}}`), "proj")
	require.NoError(t, err)
	assert.Equal(t, v1.KindDashboard, d.Kind)
	assert.Equal(t, "other", d.Metadata.Project)

	_, err = DecodeDashboard(json.RawMessage(`{"kind": "Datasource", "metadata": {"name": "d"}, "spec": {"duration": "1h"}}`), "proj")
	assert.EqualError(t, err, `invalid kind: "Datasource" for a Dashboard type`)
}
