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

package schema

import (
	"strings"
	"testing"
	"time"
)

// sampleDashboardSchema mimics /api/v1/schemas/dashboards: one self-contained definition per plugin.
const sampleDashboardSchema = `import "strings"

#Dashboard: {
	kind: "Dashboard"
	spec: panels: [string]: plugin: {
		kind: #Plugin_Panel_StatChart.kind
		spec: #Plugin_Panel_StatChart.spec
	}
}
#Plugin_Panel_StatChart: {
	let CALCULATION_3 = {
		#x: "first" | *"last" | "mean"
	}
	kind: "StatChart"
	spec: close({
		calculation: CALCULATION_3.#x
	})
}
#Plugin_Query_PrometheusTimeSeriesQuery: {
	let KIND_81 = {
		#x: "PrometheusDatasource"
	}
	kind: "PrometheusTimeSeriesQuery"
	spec: close({
		datasource?: kind: KIND_81.#x
		query: strings.MinRunes(1)
	})
}
`

const expandedStatChart = `#StatChart: {
	let CALCULATION_3 = {
		#x: "first" | *"last" | "mean"
	}
	kind: "StatChart"
	spec: close({
		calculation: CALCULATION_3.#x
	})
}
`

const expandedPrometheusTimeSeriesQuery = `import "strings"

#PrometheusTimeSeriesQuery: {
	let KIND_81 = {
		#x: "PrometheusDatasource"
	}
	kind: "PrometheusTimeSeriesQuery"
	spec: close({
		datasource?: kind: KIND_81.#x
		query: strings.MinRunes(1)
	})
}
`

func TestGetPluginSchemaExpansion(t *testing.T) {
	testSuites := []struct {
		title           string
		dashboardSchema string
		arguments       map[string]any
		isError         bool
		expected        string
		expectedPaths   []string
	}{
		{
			title:           "panel schema with its shared types",
			dashboardSchema: sampleDashboardSchema,
			arguments:       map[string]any{"kind": "StatChart"},
			expected:        expandedStatChart,
			expectedPaths:   []string{"/api/v1/schemas/dashboards"},
		},
		{
			title:           "query schema keeping the strings import",
			dashboardSchema: sampleDashboardSchema,
			arguments:       map[string]any{"kind": "PrometheusTimeSeriesQuery"},
			expected:        expandedPrometheusTimeSeriesQuery,
			expectedPaths:   []string{"/api/v1/schemas/dashboards"},
		},
		{
			title:           "version uses the plugin schema endpoint",
			dashboardSchema: sampleDashboardSchema,
			arguments:       map[string]any{"kind": "TimeSeriesChart", "version": "0.14.0"},
			expected:        timeSeriesChartSchema,
			expectedPaths:   []string{"/api/v1/schemas/plugins/TimeSeriesChart"},
		},
		{
			title:           "registry uses the plugin schema endpoint",
			dashboardSchema: sampleDashboardSchema,
			arguments:       map[string]any{"kind": "TimeSeriesChart", "registry": "perses.dev"},
			expected:        timeSeriesChartSchema,
			expectedPaths:   []string{"/api/v1/schemas/plugins/TimeSeriesChart"},
		},
		{
			title:           "kind missing from the dashboard schema falls back to the plugin schema endpoint",
			dashboardSchema: sampleDashboardSchema,
			arguments:       map[string]any{"kind": "TimeSeriesChart"},
			expected:        timeSeriesChartSchema,
			expectedPaths:   []string{"/api/v1/schemas/dashboards", "/api/v1/schemas/plugins/TimeSeriesChart"},
		},
		{
			title:         "dashboard schema error falls back to the plugin schema endpoint",
			arguments:     map[string]any{"kind": "TimeSeriesChart"},
			expected:      timeSeriesChartSchema,
			expectedPaths: []string{"/api/v1/schemas/dashboards", "/api/v1/schemas/plugins/TimeSeriesChart"},
		},
		{
			title:           "invalid dashboard schema falls back to the plugin schema endpoint",
			dashboardSchema: "#Plugin_Panel_StatChart: {",
			arguments:       map[string]any{"kind": "TimeSeriesChart"},
			expected:        timeSeriesChartSchema,
			expectedPaths:   []string{"/api/v1/schemas/dashboards", "/api/v1/schemas/plugins/TimeSeriesChart"},
		},
		{
			title:           "shared type returns a hint",
			dashboardSchema: sampleDashboardSchema,
			arguments:       map[string]any{"kind": "calculation"},
			isError:         true,
			expected:        `"calculation" is not a plugin kind; ` + notPluginKindHint,
			expectedPaths:   []string{"/api/v1/schemas/dashboards", "/api/v1/schemas/plugins/calculation"},
		},
	}

	for _, test := range testSuites {
		t.Run(test.title, func(t *testing.T) {
			perses := &fakePerses{dashboardSchema: test.dashboardSchema}
			session := connect(t, perses)
			result, text := callTool(t, session, "perses_get_plugin_schema", test.arguments)
			if result.IsError != test.isError {
				t.Fatalf("expected isError=%v, got %v: %s", test.isError, result.IsError, text)
			}
			if text != test.expected {
				t.Errorf("expected %q, got %q", test.expected, text)
			}
			if paths := strings.Join(perses.paths(), ","); paths != strings.Join(test.expectedPaths, ",") {
				t.Errorf("expected requests to %v, got %s", test.expectedPaths, paths)
			}
		})
	}
}

func TestGetPluginSchemaCache(t *testing.T) {
	perses := &fakePerses{dashboardSchema: sampleDashboardSchema}
	session, resource := connectSchema(t, perses)
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	resource.pluginBlocks.now = func() time.Time { return now }

	call := func() {
		t.Helper()
		if result, text := callTool(t, session, "perses_get_plugin_schema", map[string]any{"kind": "StatChart"}); result.IsError || text != expandedStatChart {
			t.Fatalf("unexpected result: %s", text)
		}
	}
	countFetches := func() int {
		count := 0
		for _, path := range perses.paths() {
			if path == "/api/v1/schemas/dashboards" {
				count++
			}
		}
		return count
	}

	call()
	now = now.Add(pluginBlocksTTL - time.Second)
	call()
	if fetches := countFetches(); fetches != 1 {
		t.Fatalf("expected the dashboard schema to be fetched once, got %d", fetches)
	}
	now = now.Add(time.Second)
	call()
	if fetches := countFetches(); fetches != 2 {
		t.Fatalf("expected the dashboard schema to be fetched again after expiry, got %d", fetches)
	}
}
