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
	"math"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/common/model"
)

func TestResolve(t *testing.T) {
	projectDefault := append([]testDatasource{{Name: "local", Project: "demo", Kind: prometheusDatasourceKind, Default: true}}, defaultDatasources...)
	single := []testDatasource{{Name: "only", Kind: prometheusDatasourceKind}, {Name: "logs", Kind: "LokiDatasource", Default: true}}
	several := []testDatasource{
		{Name: "a", Kind: prometheusDatasourceKind},
		{Name: "b", Kind: prometheusDatasourceKind},
		{Name: "c", Project: "demo", Kind: prometheusDatasourceKind},
		{Name: "c2", Project: "demo", Kind: prometheusDatasourceKind},
	}
	direct := []testDatasource{{Name: "browser", Kind: prometheusDatasourceKind, DirectURL: true}, {Name: "proxied", Kind: prometheusDatasourceKind}}
	directDefault := []testDatasource{{Name: "browser", Kind: prometheusDatasourceKind, DirectURL: true, Default: true}, {Name: "proxied", Kind: prometheusDatasourceKind}}
	tests := []struct {
		name        string
		datasources []testDatasource
		input       *DatasourceRef
		want        DatasourceRef
		wantErr     []string
	}{
		{name: "omitted: global default", datasources: defaultDatasources, want: DatasourceRef{Name: "prom"}},
		{name: "explicit global", datasources: defaultDatasources, input: &DatasourceRef{Name: "prom"}, want: DatasourceRef{Name: "prom"}},
		{name: "explicit project", datasources: defaultDatasources, input: &DatasourceRef{Name: "prom", Project: "demo"}, want: DatasourceRef{Name: "prom", Project: "demo"}},
		{name: "project default", datasources: projectDefault, input: &DatasourceRef{Project: "demo"}, want: DatasourceRef{Name: "local", Project: "demo"}},
		{name: "project without default falls back to the global default", datasources: defaultDatasources, input: &DatasourceRef{Project: "demo"}, want: DatasourceRef{Name: "prom"}},
		{name: "single Prometheus datasource without default", datasources: single, want: DatasourceRef{Name: "only"}},
		{name: "wrong kind", datasources: defaultDatasources, input: &DatasourceRef{Name: "logs"}, wantErr: []string{"global datasource 'logs' is a LokiDatasource, not a PrometheusDatasource"}},
		{name: "not found", datasources: defaultDatasources, input: &DatasourceRef{Name: "nope"}, wantErr: []string{"error retrieving global datasource 'nope'", "datasource.project"}},
		{name: "none", datasources: nil, wantErr: []string{"no Prometheus datasource found"}},
		{name: "directUrl datasource", datasources: direct, input: &DatasourceRef{Name: "browser"}, wantErr: []string{"global datasource 'browser' uses a directUrl", "configure a proxy on it"}},
		{name: "directUrl default datasource", datasources: directDefault, wantErr: []string{"the default Prometheus datasource can't be used: global datasource 'browser' uses a directUrl"}},
		{name: "single proxied datasource besides a directUrl one", datasources: direct, want: DatasourceRef{Name: "proxied"}},
		{name: "only directUrl datasources", datasources: direct[:1], wantErr: []string{"no Prometheus datasource can be used: global datasource 'browser' uses a directUrl"}},
		{name: "no default among several", datasources: several, input: &DatasourceRef{Project: "demo"}, wantErr: []string{"no default Prometheus datasource", "c (project demo), c2 (project demo), a (global), b (global)"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			perses := newFakePerses(t, nil)
			perses.datasources = tt.datasources
			got, err := newTestPrometheus(t, perses).resolver.resolve(tt.input)
			if len(tt.wantErr) > 0 {
				if err == nil {
					t.Fatalf("Resolve() = %+v, want an error", got)
				}
				for _, want := range tt.wantErr {
					if !strings.Contains(err.Error(), want) {
						t.Errorf("error %q does not contain %q", err, want)
					}
				}
				return
			}
			if err != nil {
				t.Fatalf("Resolve() error: %v", err)
			}
			if got.ref != tt.want {
				t.Errorf("Resolve() = %+v, want %+v", got.ref, tt.want)
			}
		})
	}
}

func TestResolvePluginSpec(t *testing.T) {
	perses := newFakePerses(t, nil)
	perses.datasources = []testDatasource{
		{Name: "set", Kind: prometheusDatasourceKind, ScrapeInterval: "30s", QueryParams: map[string]string{"dedup": "true", "tenant": "$tenant"}},
		{Name: "unset", Kind: prometheusDatasourceKind},
	}
	resolver := newTestPrometheus(t, perses).resolver
	set, err := resolver.resolve(&DatasourceRef{Name: "set"})
	if err != nil {
		t.Fatal(err)
	}
	if set.scrapeInterval != 30*time.Second || !reflect.DeepEqual(set.queryParams, map[string]string{"dedup": "true", "tenant": "$tenant"}) {
		t.Errorf("resolved = %+v", set)
	}
	if warnings := set.warnings(); len(warnings) != 1 || !strings.Contains(warnings[0], "'tenant'") {
		t.Errorf("warnings = %v, want one about tenant", warnings)
	}
	unset, err := resolver.resolve(&DatasourceRef{Name: "unset"})
	if err != nil {
		t.Fatal(err)
	}
	// Like in the Perses UI, the scrape interval defaults to 1m.
	if unset.scrapeInterval != time.Minute || unset.queryParams != nil || unset.warnings() != nil {
		t.Errorf("resolved = %+v", unset)
	}
}

func TestResolveCache(t *testing.T) {
	perses := newFakePerses(t, nil)
	resolver := newTestPrometheus(t, perses).resolver
	now := time.Now()
	resolver.now = func() time.Time { return now }
	for range 3 {
		if _, err := resolver.resolve(nil); err != nil {
			t.Fatal(err)
		}
	}
	if count := len(perses.Received()); count != 1 {
		t.Errorf("%d Perses requests, want 1", count)
	}
	now = now.Add(cacheTTL + time.Second)
	if _, err := resolver.resolve(nil); err != nil {
		t.Fatal(err)
	}
	if count := len(perses.Received()); count != 2 {
		t.Errorf("%d Perses requests after expiry, want 2", count)
	}
}

func TestResolveTimeRange(t *testing.T) {
	tests := []struct {
		name      string
		input     *TimeRangeInput
		wantStart time.Time
		wantEnd   time.Time
		wantRange timeRangeOutput
		wantErr   string
	}{
		{name: "default is the past hour", wantStart: testNow.Add(-time.Hour), wantEnd: testNow, wantRange: timeRangeOutput{PastDuration: "1h"}},
		{name: "past duration", input: &TimeRangeInput{PastDuration: "1d12h"}, wantStart: testNow.Add(-36 * time.Hour), wantEnd: testNow, wantRange: timeRangeOutput{PastDuration: "1d12h"}},
		{
			name:      "absolute range",
			input:     &TimeRangeInput{Start: "2026-01-02T10:00:00+01:00", End: "2026-01-02T10:30:00Z"},
			wantStart: time.Date(2026, 1, 2, 9, 0, 0, 0, time.UTC), wantEnd: time.Date(2026, 1, 2, 10, 30, 0, 0, time.UTC),
			wantRange: timeRangeOutput{Start: "2026-01-02T09:00:00Z", End: "2026-01-02T10:30:00Z"},
		},
		{
			name:      "absolute range ends now by default",
			input:     &TimeRangeInput{Start: "2026-01-02T11:00:00Z"},
			wantStart: time.Date(2026, 1, 2, 11, 0, 0, 0, time.UTC), wantEnd: testNow,
			wantRange: timeRangeOutput{Start: "2026-01-02T11:00:00Z", End: "2026-01-02T12:00:00Z"},
		},
		{
			name:      "fractional seconds are kept",
			input:     &TimeRangeInput{Start: "2026-01-02T10:00:00.100Z", End: "2026-01-02T10:00:00.900Z"},
			wantStart: time.Date(2026, 1, 2, 10, 0, 0, 100_000_000, time.UTC), wantEnd: time.Date(2026, 1, 2, 10, 0, 0, 900_000_000, time.UTC),
			wantRange: timeRangeOutput{Start: "2026-01-02T10:00:00.1Z", End: "2026-01-02T10:00:00.9Z"},
		},
		{name: "invalid past duration", input: &TimeRangeInput{PastDuration: "an hour"}, wantErr: "invalid time_range.past_duration 'an hour'"},
		{name: "zero past duration", input: &TimeRangeInput{PastDuration: "0m"}, wantErr: "must be positive"},
		{name: "invalid start", input: &TimeRangeInput{Start: "2026-01-02"}, wantErr: "invalid time_range.start '2026-01-02'"},
		{name: "invalid end", input: &TimeRangeInput{Start: "2026-01-02T11:00:00Z", End: "now"}, wantErr: "invalid time_range.end 'now'"},
		{name: "start after end", input: &TimeRangeInput{Start: "2026-01-02T13:00:00Z"}, wantErr: "time_range.start must be before time_range.end"},
		{name: "end without start", input: &TimeRangeInput{End: "2026-01-02T11:00:00Z"}, wantErr: "time_range.end requires time_range.start"},
		{name: "past duration and start", input: &TimeRangeInput{PastDuration: "1h", Start: "2026-01-02T11:00:00Z"}, wantErr: "not both"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start, end, timeRange, err := resolveTimeRange(tt.input, testNow)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !start.Equal(tt.wantStart) || !end.Equal(tt.wantEnd) || timeRange != tt.wantRange {
				t.Errorf("got %s - %s %+v, want %s - %s %+v", start, end, timeRange, tt.wantStart, tt.wantEnd, tt.wantRange)
			}
		})
	}
}

func TestResolveStep(t *testing.T) {
	tests := []struct {
		step    string
		span    time.Duration
		minStep time.Duration
		want    time.Duration
	}{
		{span: time.Hour, minStep: 15 * time.Second, want: 15 * time.Second},
		// The default step is at least the scrape interval of the datasource.
		{span: time.Hour, minStep: time.Minute, want: time.Minute},
		{span: 6 * time.Hour, minStep: 15 * time.Second, want: 44 * time.Second},
		{span: 7 * 24 * time.Hour, minStep: time.Minute, want: 1210 * time.Second},
		// An explicit step is kept, even below the scrape interval.
		{step: "1m", span: time.Hour, minStep: 5 * time.Minute, want: time.Minute},
		// The step is raised to stay under the Prometheus limit of points.
		{step: "1s", span: 7 * 24 * time.Hour, minStep: time.Minute, want: 55 * time.Second},
		// A subsecond step under the limit is kept (601 points).
		{step: "100ms", span: time.Minute, minStep: time.Minute, want: 100 * time.Millisecond},
		// A subsecond step over the limit is raised to the smallest step under it, to the millisecond (60s / 11000).
		{step: "1ms", span: time.Minute, minStep: time.Minute, want: 6 * time.Millisecond},
	}
	for _, tt := range tests {
		got, err := resolveStep(tt.step, testNow.Add(-tt.span), testNow, tt.minStep)
		if err != nil || got != tt.want {
			t.Errorf("resolveStep(%q, %s, %s) = %s, %v, want %s", tt.step, tt.span, tt.minStep, got, err, tt.want)
		}
	}
}

func TestEscapeLabelName(t *testing.T) {
	tests := []struct{ label, want string }{
		{label: "job", want: "job"},
		{label: "__name__", want: "__name__"},
		{label: "http.method", want: "U__http_2e_method"},
		{label: "a/b", want: "U__a_2f_b"},
		// Prometheus unescapes every name starting with U__: such names are escaped to be read back unchanged.
		{label: "U__job", want: "U__U____job"},
		{label: "U__http.method", want: "U__U____http_2e_method"},
	}
	for _, tt := range tests {
		got := escapeLabelName(tt.label)
		if got != tt.want {
			t.Errorf("escapeLabelName(%q) = %q, want %q", tt.label, got, tt.want)
		}
		// The label values endpoint of Prometheus unescapes the names starting with U__ only.
		read := got
		if strings.HasPrefix(read, "U__") {
			read = model.UnescapeName(read, model.ValueEncodingEscaping)
		}
		if read != tt.label {
			t.Errorf("Prometheus reads %q as %q, want %q", got, read, tt.label)
		}
	}
}

func TestSeriesStats(t *testing.T) {
	samples := []model.SamplePair{
		{Timestamp: 1767355200000, Value: 2},
		{Timestamp: 1767355215000, Value: model.SampleValue(math.NaN())},
		{Timestamp: 1767355230000, Value: 1},
		{Timestamp: 1767355245000, Value: 3.33333},
		{Timestamp: 1767355260000, Value: model.SampleValue(math.Inf(1))},
	}
	got, maxValue, ok := seriesStats(samples, time.UTC)
	want := stats{Min: 1, Max: 3.333, Avg: 2.111, Last: "+Inf", MinAt: "2 Jan 2026, 12:00", MaxAt: "2 Jan 2026, 12:00", NonFinite: 2}
	if !ok || !reflect.DeepEqual(got, want) || maxValue != 3.33333 {
		t.Errorf("seriesStats() = %+v, %v, %v, want %+v", got, maxValue, ok, want)
	}
	if _, _, ok := seriesStats([]model.SamplePair{{Value: model.SampleValue(math.Inf(1))}}, time.UTC); ok {
		t.Error("seriesStats() of a series without finite values is ok")
	}
	// The average of large finite values doesn't overflow.
	large := []model.SamplePair{{Value: math.MaxFloat64}, {Value: math.MaxFloat64}, {Value: -math.MaxFloat64}}
	if got, _, _ := seriesStats(large, time.UTC); math.IsInf(got.Avg, 0) || math.Abs(got.Avg/(math.MaxFloat64/3)-1) > 1e-9 {
		t.Errorf("average of large values = %v, want about %v", got.Avg, math.MaxFloat64/3)
	}
}

func TestKeepTopSeries(t *testing.T) {
	all := []series{
		{Labels: map[string]string{"job": "p", "handler": "/a"}, sortKey: 1},
		{Labels: map[string]string{"job": "p", "handler": "/b"}, sortKey: 3},
		{Labels: map[string]string{"job": "p", "handler": "/c"}, sortKey: 2},
	}
	top, common := keepTopSeries(all, 2)
	if !reflect.DeepEqual(common, map[string]string{"job": "p"}) {
		t.Errorf("common = %v", common)
	}
	if len(top) != 2 || !reflect.DeepEqual(top[0].Labels, map[string]string{"handler": "/b"}) || !reflect.DeepEqual(top[1].Labels, map[string]string{"handler": "/c"}) {
		t.Errorf("top = %+v", top)
	}

	single, common := keepTopSeries([]series{{Labels: map[string]string{"a": "1"}}}, 5)
	if common != nil || single[0].Labels["a"] != "1" {
		t.Errorf("single series: common = %v, labels = %v", common, single[0].Labels)
	}
	_, common = keepTopSeries([]series{{Labels: map[string]string{"a": "1"}}, {Labels: map[string]string{}}}, 5)
	if common != nil {
		t.Errorf("common = %v, want none (a series has no labels)", common)
	}
}

func TestKeepTopSeriesSortsNaNLast(t *testing.T) {
	all, err := vectorSeries(model.Vector{
		{Metric: model.Metric{"s": "nan"}, Value: model.SampleValue(math.NaN())},
		{Metric: model.Metric{"s": "-inf"}, Value: model.SampleValue(math.Inf(-1))},
		{Metric: model.Metric{"s": "one"}, Value: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	// NaN comes first in the result of Prometheus but is dropped: -Inf is lower than any number, NaN is lower still.
	top, _ := keepTopSeries(all, 2)
	if len(top) != 2 || top[0].Labels["s"] != "one" || top[1].Labels["s"] != "-inf" || top[1].Value != "-Inf" {
		t.Errorf("top = %+v, want one and -inf", top)
	}
}

func TestRound(t *testing.T) {
	tests := []struct{ value, want float64 }{
		{value: 12.34567, want: 12.35},
		{value: 0.000123456, want: 0.0001235},
		{value: 1234.567, want: 1235},
		{value: 123456.789, want: 123457},
		{value: -98765.4, want: -98765},
		// A timestamp keeps all its digits: with 4 significant digits, it would be 1767000000, 4 days earlier.
		{value: 1767355200.123, want: 1767355200},
	}
	for _, tt := range tests {
		if got := round(tt.value); got != tt.want {
			t.Errorf("round(%v) = %v, want %v", tt.value, got, tt.want)
		}
	}
}

func TestResolveEmptyScrapeInterval(t *testing.T) {
	proxySpec := map[string]any{"kind": "HTTPProxy", "spec": map[string]any{"url": "http://prometheus.example.com"}}
	perses := newFakePerses(t, nil)
	perses.datasources = []testDatasource{
		// Perses accepts an empty scrape interval: the UI saves it when the field is cleared.
		{Name: "cleared", Kind: prometheusDatasourceKind, PluginSpec: map[string]any{"proxy": proxySpec, "scrapeInterval": ""}},
		{Name: "invalid", Kind: prometheusDatasourceKind, PluginSpec: map[string]any{"proxy": proxySpec, "scrapeInterval": "fast"}},
	}
	resolver := newTestPrometheus(t, perses).resolver
	cleared, err := resolver.resolve(&DatasourceRef{Name: "cleared"})
	if err != nil {
		t.Fatalf("resolve() error: %v", err)
	}
	if cleared.scrapeInterval != defaultScrapeInterval {
		t.Errorf("scrape interval = %s, want the default %s", cleared.scrapeInterval, defaultScrapeInterval)
	}

	// Without default, the error of a datasource that can't be used is its own, not a missing proxy.
	perses.datasources = perses.datasources[1:]
	_, err = resolver.resolve(nil)
	if err == nil || !strings.Contains(err.Error(), "invalid plugin spec of global datasource 'invalid'") || strings.Contains(err.Error(), "proxy") {
		t.Errorf("error = %v, want the invalid plugin spec", err)
	}
}

func TestResolveProjectDefaultWithoutGlobalAccess(t *testing.T) {
	perses := newFakePerses(t, nil)
	perses.datasources = []testDatasource{{Name: "local", Project: "demo", Kind: prometheusDatasourceKind, Default: true}}
	perses.forbidGlobalList = true
	got, err := newTestPrometheus(t, perses).resolver.resolve(&DatasourceRef{Project: "demo"})
	if err != nil {
		t.Fatalf("resolve() error: %v", err)
	}
	if got.ref != (DatasourceRef{Name: "local", Project: "demo"}) {
		t.Errorf("resolve() = %+v", got.ref)
	}
	for _, request := range perses.Received() {
		if strings.Contains(request.Path, "globaldatasources") {
			t.Errorf("request %s %s: the global datasources are listed although the project has a default", request.Method, request.Path)
		}
	}
}

func TestResolveCacheRefreshesTheSettings(t *testing.T) {
	perses := newFakePerses(t, nil)
	perses.datasources = []testDatasource{{Name: "prom", Kind: prometheusDatasourceKind, Default: true, ScrapeInterval: "30s", QueryParams: map[string]string{"dedup": "true"}}}
	resolver := newTestPrometheus(t, perses).resolver
	now := time.Now()
	resolver.now = func() time.Time { return now }
	if _, err := resolver.resolve(nil); err != nil {
		t.Fatal(err)
	}
	perses.datasources = []testDatasource{{Name: "prom", Kind: prometheusDatasourceKind, Default: true, ScrapeInterval: "2m", QueryParams: map[string]string{"dedup": "false"}}}
	cached, err := resolver.resolve(nil)
	if err != nil {
		t.Fatal(err)
	}
	if cached.scrapeInterval != 30*time.Second || cached.queryParams["dedup"] != "true" {
		t.Errorf("cached = %+v, want the settings of the first resolution", cached)
	}
	now = now.Add(cacheTTL + time.Second)
	refreshed, err := resolver.resolve(nil)
	if err != nil {
		t.Fatal(err)
	}
	if refreshed.scrapeInterval != 2*time.Minute || refreshed.queryParams["dedup"] != "false" {
		t.Errorf("refreshed = %+v, want the new settings", refreshed)
	}
}
