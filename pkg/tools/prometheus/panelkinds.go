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

package prometheus

import (
	"fmt"
	"maps"
	"slices"
	"strings"
	"sync"
	"time"

	v1 "github.com/perses/perses/pkg/model/api/v1"
	"github.com/perses/spec/go/dashboard"
	"github.com/perses/spec/go/plugin"
)

const (
	defaultPanelKind = "TimeSeriesChart"
	// timeSeriesQueryKind and prometheusQueryKind are the kinds of the panel queries.
	timeSeriesQueryKind = "TimeSeriesQuery"
	prometheusQueryKind = "PrometheusTimeSeriesQuery"
	// installedPanelsTTL is how long the list of the panel plugins installed in Perses is reused.
	installedPanelsTTL = 5 * time.Minute
)

// panelKind is a panel plugin bundled in the view of perses_render_panels.
type panelKind struct {
	// defaults returns the plugin spec of a new panel, valid against the plugin schema.
	defaults func() map[string]any
	// unitPath is the path of the unit in the plugin spec. Empty when the panel has no unit.
	unitPath []string
}

// panelKinds are the panel plugins of perses/plugins that display time series queries. The view bundles them: keep
// this list in sync with ui/mcp-apps/app/plugins.ts.
var panelKinds = map[string]panelKind{
	"TimeSeriesChart": {
		defaults: func() map[string]any {
			return map[string]any{
				"legend": map[string]any{"position": "bottom", "mode": "list"},
				"visual": map[string]any{"display": "line", "lineWidth": 1.25, "areaOpacity": 0.1},
			}
		},
		unitPath: []string{"yAxis", "format", "unit"},
	},
	"StatChart": {
		defaults: func() map[string]any {
			return map[string]any{"calculation": "last-number", "sparkline": map[string]any{}}
		},
		unitPath: []string{"format", "unit"},
	},
	"GaugeChart": {
		defaults: func() map[string]any { return map[string]any{"calculation": "last-number"} },
		unitPath: []string{"format", "unit"},
	},
	"BarChart": {
		defaults: func() map[string]any {
			return map[string]any{"calculation": "last-number", "sort": "desc", "mode": "value"}
		},
		unitPath: []string{"format", "unit"},
	},
	"PieChart": {
		defaults: func() map[string]any {
			return map[string]any{
				"calculation": "last-number", "radius": 50, "sort": "desc", "mode": "value",
				"legend": map[string]any{"position": "right", "mode": "list"},
				// Without a palette, the plugin picks a color from each series name, and two slices can get the same
				// color. This is the categorical palette of the Perses charts theme.
				"colorPalette": []any{"#56B4E9", "#009E73", "#0072B2", "#CC79A7", "#F0E442", "#E69F00", "#D55E00"},
			}
		},
		unitPath: []string{"format", "unit"},
	},
	"Table": {
		defaults: func() map[string]any { return map[string]any{} },
	},
	"TimeSeriesTable": {
		defaults: func() map[string]any { return map[string]any{} },
	},
	"HeatMapChart": {
		defaults: func() map[string]any { return map[string]any{} },
		unitPath: []string{"yAxisFormat", "unit"},
	},
	"HistogramChart": {
		defaults: func() map[string]any { return map[string]any{} },
		unitPath: []string{"format", "unit"},
	},
	"StatusHistoryChart": {
		defaults: func() map[string]any { return map[string]any{} },
	},
}

// nativeHistogramKinds are the panel kinds that display native histograms only.
var nativeHistogramKinds = map[string]bool{"HeatMapChart": true, "HistogramChart": true}

// panelKindNames returns the names of the bundled panel kinds, sorted.
func panelKindNames() []string {
	return slices.Sorted(maps.Keys(panelKinds))
}

// units are the units of the Perses formats (common.#format of the plugin schemas).
var units = []string{
	"decimal",
	"percent", "percent-decimal",
	"nanoseconds", "microseconds", "milliseconds", "seconds", "minutes", "hours", "days", "weeks", "months", "years",
	"bits", "decbits", "bytes", "decbytes",
	"bits/sec", "decbits/sec", "bytes/sec", "decbytes/sec", "counts/sec", "events/sec", "messages/sec", "ops/sec",
	"packets/sec", "reads/sec", "records/sec", "requests/sec", "rows/sec", "writes/sec",
	"aud", "cad", "chf", "cny", "eur", "gbp", "hkd", "inr", "jpy", "krw", "nok", "nzd", "sek", "sgd", "usd",
	"celsius", "fahrenheit",
	"datetime-iso", "datetime-us", "datetime-local", "date-iso", "date-us", "date-local", "time-local", "time-iso",
	"time-us", "relative-time", "unix-timestamp", "unix-timestamp-ms",
}

// buildPanel returns the Perses definition of a panel: the defaults of its kind, its unit, then its spec merged over
// them. The kind and unit must have been checked.
func buildPanel(input PanelInput, kind string, datasource DatasourceRef) *dashboard.Panel {
	definition := panelKinds[kind]
	spec := definition.defaults()
	if input.Unit != "" {
		setPath(spec, definition.unitPath, input.Unit)
	}
	mergeSpec(spec, input.Spec)

	selector := map[string]any{"kind": prometheusDatasourceKind, "name": datasource.Name}
	queries := make([]dashboard.Query, 0, len(input.Queries))
	for _, query := range input.Queries {
		querySpec := map[string]any{"datasource": selector, "query": query.PromQL}
		if query.Legend != "" {
			querySpec["seriesNameFormat"] = query.Legend
		}
		queries = append(queries, dashboard.Query{
			Kind: timeSeriesQueryKind,
			Spec: dashboard.QuerySpec{Plugin: plugin.Plugin{Kind: prometheusQueryKind, Spec: querySpec}},
		})
	}
	return &dashboard.Panel{
		Kind: "Panel",
		Spec: dashboard.PanelSpec{
			Display: &dashboard.PanelDisplay{Name: input.Title, Description: input.Description},
			Plugin:  plugin.Plugin{Kind: kind, Spec: spec},
			Queries: queries,
		},
	}
}

// setPath sets value at path in spec, creating the intermediate objects.
func setPath(spec map[string]any, path []string, value any) {
	for _, key := range path[:len(path)-1] {
		child, ok := spec[key].(map[string]any)
		if !ok {
			child = map[string]any{}
			spec[key] = child
		}
		spec = child
	}
	spec[path[len(path)-1]] = value
}

// mergeSpec merges override into spec: objects are merged recursively, the other values replaced.
func mergeSpec(spec, override map[string]any) {
	for key, value := range override {
		overrideChild, isObject := value.(map[string]any)
		specChild, specIsObject := spec[key].(map[string]any)
		if isObject && specIsObject {
			mergeSpec(specChild, overrideChild)
			continue
		}
		spec[key] = value
	}
}

// checkPanelKind returns the kind of a panel (the default one when empty), or an error when the view doesn't bundle
// it or Perses doesn't have it. installed is nil when the installed plugins are unknown.
func checkPanelKind(kind string, installed map[string]bool) (string, error) {
	if kind == "" {
		kind = defaultPanelKind
	}
	if _, bundled := panelKinds[kind]; !bundled {
		return "", fmt.Errorf("unsupported panel kind '%s': use one of %s", kind, strings.Join(availableKinds(installed), ", "))
	}
	if installed != nil && !installed[kind] {
		return "", fmt.Errorf("the panel plugin '%s' is not installed in Perses: use one of %s", kind, strings.Join(availableKinds(installed), ", "))
	}
	return kind, nil
}

// availableKinds returns the bundled panel kinds that are installed in Perses (all when installed is nil).
func availableKinds(installed map[string]bool) []string {
	var kinds []string
	for _, kind := range panelKindNames() {
		if installed == nil || installed[kind] {
			kinds = append(kinds, kind)
		}
	}
	return kinds
}

// checkUnit returns an error when unit is not a Perses unit or the panel kind has no unit.
func checkUnit(unit, kind string) error {
	if unit == "" {
		return nil
	}
	if !slices.Contains(units, unit) {
		return fmt.Errorf("unknown unit '%s': use one of %s", unit, strings.Join(units, ", "))
	}
	if len(panelKinds[kind].unitPath) == 0 {
		return fmt.Errorf("%s panels have no unit: remove unit", kind)
	}
	return nil
}

// installedPanels lists the panel plugins installed in Perses and caches them for installedPanelsTTL.
type installedPanels struct {
	list    func() ([]v1.PluginModule, error)
	now     func() time.Time
	mutex   sync.Mutex
	kinds   map[string]bool
	expires time.Time
}

// get returns the installed panel kinds. Errors are not cached.
func (i *installedPanels) get() (map[string]bool, error) {
	i.mutex.Lock()
	defer i.mutex.Unlock()
	if i.kinds != nil && i.now().Before(i.expires) {
		return i.kinds, nil
	}
	modules, err := i.list()
	if err != nil {
		return nil, err
	}
	kinds := map[string]bool{}
	for _, module := range modules {
		if module.Status != nil && !module.Status.IsLoaded {
			continue
		}
		for _, p := range module.Spec.Plugins {
			if p.Kind == plugin.KindPanel {
				kinds[p.Spec.Name] = true
			}
		}
	}
	i.kinds = kinds
	i.expires = i.now().Add(installedPanelsTTL)
	return kinds, nil
}
