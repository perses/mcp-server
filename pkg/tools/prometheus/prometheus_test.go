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
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"github.com/perses/mcp-server/pkg/tools"
	"github.com/perses/mcp-server/pkg/tools/internal/tooltest"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
)

func TestGetTools(t *testing.T) {
	want := []string{ListMetricNamesToolName, ListLabelNamesToolName, ListLabelValuesToolName, GetMetricMetadataToolName, QueryToolName}
	got := New(nil).GetTools()
	if len(got) != len(want) {
		t.Fatalf("got %d tools, want %d", len(got), len(want))
	}
	for i, tool := range got {
		if tool.MCPTool.Name != want[i] || tool.IsWriteTool || tool.ResourceType != tools.PrometheusResource {
			t.Errorf("tool %d = %s (write %v, resource %s)", i, tool.MCPTool.Name, tool.IsWriteTool, tool.ResourceType)
		}
		annotations := tool.MCPTool.Annotations
		if !annotations.ReadOnlyHint || *annotations.DestructiveHint || !annotations.IdempotentHint || *annotations.OpenWorldHint {
			t.Errorf("tool %s annotations = %+v", tool.MCPTool.Name, annotations)
		}
	}
}

func TestListMetricNames(t *testing.T) {
	perses := newFakePerses(t, map[string]answer{
		globalProxy + "/api/v1/label/__name__/values": ok(`["go_memstats_heap_inuse_bytes","process_resident_memory_bytes","up","go_memstats_sys_bytes","x_memstats_seconds_bucket"]`),
	})
	perses.metadata = map[string][]v1.Metadata{
		"go_memstats_heap_inuse_bytes": {{Type: v1.MetricTypeGauge, Help: strings.Repeat("h", 200)}},
		"x_memstats_seconds":           {{Type: v1.MetricTypeHistogram, Help: "h"}},
	}
	session := connect(t, perses)

	output := decode(t, call(t, session, ListMetricNamesToolName, map[string]any{
		"search": "MEMSTATS", "match": []any{`{job="prometheus"}`, "up"}, "limit": 1, "include_metadata": true,
	}, false))
	if output["total"] != 3.0 || output["returned"] != 1.0 || output["truncated"] != true {
		t.Errorf("output = %v", output)
	}
	if !reflect.DeepEqual(output["datasource"], map[string]any{"name": "prom"}) {
		t.Errorf("datasource = %v", output["datasource"])
	}
	metric := output["metrics"].([]any)[0].(map[string]any)
	if metric["name"] != "go_memstats_heap_inuse_bytes" || metric["type"] != "gauge" || len([]rune(metric["help"].(string))) != maxHelpLength {
		t.Errorf("metric = %v", metric)
	}
	params := perses.paramsOf(globalProxy + "/api/v1/label/__name__/values")
	if !reflect.DeepEqual(params["match[]"], []string{`{job="prometheus"}`, "up"}) {
		t.Errorf("match[] = %v", params["match[]"])
	}
	if params.Get("start") != "1767351600" || params.Get("end") != "1767355200" {
		t.Errorf("time range = %s - %s", params.Get("start"), params.Get("end"))
	}

	output = decode(t, call(t, session, ListMetricNamesToolName, map[string]any{"search": "memstats", "include_metadata": true}, false))
	if bucket := output["metrics"].([]any)[2].(map[string]any); bucket["type"] != "histogram" {
		t.Errorf("histogram bucket = %v", bucket)
	}

	// A single selector is accepted as a string.
	output = decode(t, call(t, session, ListMetricNamesToolName, map[string]any{"match": "up"}, false))
	if output["total"] != 5.0 || output["truncated"] != false {
		t.Errorf("output = %v", output)
	}
	if _, isString := output["metrics"].([]any)[0].(string); !isString {
		t.Errorf("metrics without metadata = %v", output["metrics"])
	}
	if got := perses.paramsOf(globalProxy + "/api/v1/label/__name__/values")["match[]"]; !reflect.DeepEqual(got, []string{"up"}) {
		t.Errorf("match[] = %v", got)
	}
}

func TestListLabels(t *testing.T) {
	perses := newFakePerses(t, map[string]answer{
		projectProxy + "/api/v1/labels":           ok(`["__name__","instance","job"]`),
		projectProxy + "/api/v1/label/job/values": ok(`["avalanche","perses","prometheus"]`),
	})
	session := connect(t, perses)
	datasource := map[string]any{"name": "prom", "project": "demo"}

	output := decode(t, call(t, session, ListLabelNamesToolName, map[string]any{"datasource": datasource, "limit": 2}, false))
	if !reflect.DeepEqual(output["labels"], []any{"__name__", "instance"}) || output["total"] != 3.0 || output["truncated"] != true {
		t.Errorf("label names = %v", output)
	}

	output = decode(t, call(t, session, ListLabelValuesToolName, map[string]any{
		"datasource": datasource, "label": "job", "search": "PER", "time_range": map[string]any{"past_duration": "6h"},
	}, false))
	if !reflect.DeepEqual(output["values"], []any{"perses"}) || output["label"] != "job" || output["total"] != 1.0 {
		t.Errorf("label values = %v", output)
	}
	if !reflect.DeepEqual(output["datasource"], datasource) {
		t.Errorf("datasource = %v", output["datasource"])
	}
	if got := perses.paramsOf(projectProxy + "/api/v1/label/job/values").Get("start"); got != "1767333600" {
		t.Errorf("start = %s", got)
	}

	call(t, session, ListLabelValuesToolName, map[string]any{"label": "bad-label"}, true)
}

func TestProxyRequestsUseThePersesCredentials(t *testing.T) {
	perses := newFakePerses(t, map[string]answer{globalProxy + "/api/v1/labels": ok(`["job"]`)})
	call(t, connect(t, perses), ListLabelNamesToolName, nil, false)
	received := perses.Received()
	if len(received) == 0 {
		t.Fatal("no request received")
	}
	for _, request := range received {
		if request.Authorization != tooltest.Authorization {
			t.Errorf("%s %s: Authorization = %q, want %q", request.Method, request.Path, request.Authorization, tooltest.Authorization)
		}
	}
}

func TestListMetricNamesWithUnavailableMetadata(t *testing.T) {
	perses := newFakePerses(t, map[string]answer{
		globalProxy + "/api/v1/label/__name__/values": ok(`["up"]`),
		globalProxy + "/api/v1/metadata":              {status: http.StatusBadGateway, body: "upstream down"},
	})
	output := decode(t, call(t, connect(t, perses), ListMetricNamesToolName, map[string]any{"include_metadata": true}, false))
	if !reflect.DeepEqual(output["metrics"], []any{map[string]any{"name": "up"}}) {
		t.Errorf("metrics = %v, want the names as objects", output["metrics"])
	}
	warnings, _ := output["warnings"].([]any)
	if len(warnings) != 1 || !strings.Contains(warnings[0].(string), "HTTP 502 Bad Gateway: upstream down") || !strings.Contains(warnings[0].(string), GetMetricMetadataToolName) {
		t.Errorf("warnings = %v", output["warnings"])
	}
}

func TestGetMetricMetadata(t *testing.T) {
	perses := newFakePerses(t, nil)
	perses.metadata = map[string][]v1.Metadata{
		"up":          {{Type: v1.MetricTypeGauge, Help: "Target is up."}},
		"queue_count": {{Type: v1.MetricTypeGauge, Help: "Items in the queue."}},
		"x_seconds":   {{Type: v1.MetricTypeHistogram, Help: "Latency."}},
		"requests":    {{Type: v1.MetricTypeCounter, Help: "Requests."}},
		"temperature": {{Type: v1.MetricTypeGauge, Help: "Temperature."}},
	}
	output := decode(t, call(t, connect(t, perses), GetMetricMetadataToolName, map[string]any{
		"metrics": []any{"up", "queue_count", "x_seconds_bucket", "requests_total", "temperature_total", "nope"},
	}, false))
	want := map[string]any{
		"up":               []any{map[string]any{"type": "gauge", "help": "Target is up.", "unit": ""}},
		"queue_count":      []any{map[string]any{"type": "gauge", "help": "Items in the queue.", "unit": ""}},
		"x_seconds_bucket": []any{map[string]any{"type": "histogram", "help": "Latency.", "unit": ""}},
		"requests_total":   []any{map[string]any{"type": "counter", "help": "Requests.", "unit": ""}},
	}
	if !reflect.DeepEqual(output["metadata"], want) {
		t.Errorf("metadata =\n%v\nwant\n%v", output["metadata"], want)
	}
	// temperature is a gauge: it is not the family of temperature_total.
	if !reflect.DeepEqual(output["missing"], []any{"temperature_total", "nope"}) {
		t.Errorf("missing = %v", output["missing"])
	}
	// A metric with metadata under its own name is looked up once.
	if count := countRequests(perses, globalProxy+"/api/v1/metadata", "metric", "queue_count") + countRequests(perses, globalProxy+"/api/v1/metadata", "metric", "queue"); count != 1 {
		t.Errorf("%d metadata requests for queue_count, want 1", count)
	}
}

// countRequests returns the number of requests of path with the query parameter key set to value.
func countRequests(perses *fakePerses, path, key, value string) int {
	count := 0
	for _, request := range perses.Received() {
		if params, err := url.ParseQuery(request.RawQuery); err == nil && request.Path == path && params.Get(key) == value {
			count++
		}
	}
	return count
}

func TestQueryInstant(t *testing.T) {
	perses := newFakePerses(t, map[string]answer{
		globalProxy + "/api/v1/query": ok(`{"resultType":"vector","result":[` +
			`{"metric":{"__name__":"x","job":"p","handler":"/a"},"value":[1767355200,"0.5"]},` +
			`{"metric":{"__name__":"x","job":"p","handler":"/b"},"value":[1767355200,"NaN"]},` +
			`{"metric":{"__name__":"x","job":"p","handler":"/c"},"value":[1767355200,"12.34567"]},` +
			`{"metric":{"__name__":"x","job":"p","handler":"/d"},"value":[1767355200,"0"]}]},"warnings":["careful"]`),
	})
	output := decode(t, call(t, connect(t, perses), QueryToolName, map[string]any{"query": "x", "limit": 3, "time": "2026-01-02T11:00:00+01:00"}, false))
	want := map[string]any{
		"datasource":   map[string]any{"name": "prom"},
		"resultType":   "vector",
		"time":         "2026-01-02T10:00:00Z",
		"seriesCount":  4.0,
		"returned":     3.0,
		"commonLabels": map[string]any{"__name__": "x", "job": "p"},
		"series": []any{
			map[string]any{"labels": map[string]any{"handler": "/c"}, "value": 12.35},
			map[string]any{"labels": map[string]any{"handler": "/a"}, "value": 0.5},
			map[string]any{"labels": map[string]any{"handler": "/d"}, "value": 0.0},
		},
		"warnings": []any{"careful"},
	}
	if !reflect.DeepEqual(output, want) {
		t.Errorf("output =\n%v\nwant\n%v", output, want)
	}
	params := perses.paramsOf(globalProxy + "/api/v1/query")
	if params.Get("time") != "1767348000" || params.Get("query") != "x" {
		t.Errorf("params = %v", params)
	}
}

func TestQueryRange(t *testing.T) {
	perses := newFakePerses(t, map[string]answer{
		globalProxy + "/api/v1/query_range": ok(`{"resultType":"matrix","result":[` +
			`{"metric":{"job":"a"},"values":[[1767355200,"1"],[1767355260,"3"]]},` +
			`{"metric":{"job":"b"},"values":[[1767355200,"5"],[1767355260,"4"]]},` +
			`{"metric":{"job":"c"},"values":[[1767355200,"NaN"]]}]}`),
	})
	session := connect(t, perses)
	output := decode(t, call(t, session, QueryToolName, map[string]any{
		"query": "up", "type": "range", "time_range": map[string]any{"past_duration": "6h"}, "time_zone": "Europe/Berlin",
	}, false))
	want := map[string]any{
		"datasource": map[string]any{"name": "prom"},
		"resultType": "matrix",
		"timeRange":  map[string]any{"pastDuration": "6h"},
		// The datasource sets no scrape interval: the step is at least 1m, its default in the Perses UI.
		"step":        "1m",
		"seriesCount": 3.0,
		"returned":    2.0,
		"series": []any{
			map[string]any{"labels": map[string]any{"job": "b"}, "min": 4.0, "max": 5.0, "avg": 4.5, "last": 4.0, "minAt": "2 Jan 2026, 13:01", "maxAt": "2 Jan 2026, 13:00"},
			map[string]any{"labels": map[string]any{"job": "a"}, "min": 1.0, "max": 3.0, "avg": 2.0, "last": 3.0, "minAt": "2 Jan 2026, 13:00", "maxAt": "2 Jan 2026, 13:01"},
		},
		"warnings": []any{"1 series are not returned: they only have NaN or infinite samples in the time range"},
	}
	if !reflect.DeepEqual(output, want) {
		t.Errorf("output =\n%v\nwant\n%v", output, want)
	}
	params := perses.paramsOf(globalProxy + "/api/v1/query_range")
	if params.Get("step") != "60" || params.Get("start") != "1767333600" || params.Get("end") != "1767355200" {
		t.Errorf("params = %v", params)
	}

	output = decode(t, call(t, session, QueryToolName, map[string]any{
		"query": "up", "type": "range", "step": "5m",
		"time_range": map[string]any{"start": "2026-01-02T10:00:00Z", "end": "2026-01-02T11:00:00Z"},
	}, false))
	if output["step"] != "5m" || !reflect.DeepEqual(output["timeRange"], map[string]any{"start": "2026-01-02T10:00:00Z", "end": "2026-01-02T11:00:00Z"}) {
		t.Errorf("output = %v", output)
	}
	if got := perses.paramsOf(globalProxy + "/api/v1/query_range").Get("step"); got != "300" {
		t.Errorf("explicit step = %s", got)
	}
}

func TestQueryScalar(t *testing.T) {
	perses := newFakePerses(t, map[string]answer{globalProxy + "/api/v1/query": ok(`{"resultType":"scalar","result":[1767355200,"42.123456"]}`)})
	output := decode(t, call(t, connect(t, perses), QueryToolName, map[string]any{"query": "scalar(up)"}, false))
	if output["resultType"] != "scalar" || output["value"] != 42.12 {
		t.Errorf("output = %v", output)
	}
	if _, found := output["series"]; found {
		t.Errorf("scalar output has series: %v", output)
	}
}

func TestQueryErrors(t *testing.T) {
	large := `{"status":"success","data":{"resultType":"vector","result":[` + strings.Repeat(" ", maxBodyBytes) + `]}}`
	tests := []struct {
		name      string
		answer    *answer
		arguments map[string]any
		want      string
	}{
		{
			name:   "bad PromQL is returned unchanged",
			answer: &answer{status: http.StatusBadRequest, body: `{"status":"error","errorType":"bad_data","error":"invalid parameter \"query\": 1:5: parse error: unclosed left parenthesis"}`},
			want:   `bad_data: invalid parameter "query": 1:5: parse error: unclosed left parenthesis`,
		},
		{
			name:   "Perses error keeps its message",
			answer: &answer{status: http.StatusNotFound, body: `{"message":"document not found: unable to forward the request to the datasource"}`},
			want:   "error querying global datasource 'prom': HTTP 404 Not Found: document not found: unable to forward the request to the datasource",
		},
		{
			name:   "Perses bad request keeps its message",
			answer: &answer{status: http.StatusBadRequest, body: `{"message":"invalid datasource proxy request"}`},
			want:   "error querying global datasource 'prom': HTTP 400 Bad Request: invalid datasource proxy request",
		},
		{
			name:   "Prometheus unavailable error is returned unchanged",
			answer: &answer{status: http.StatusServiceUnavailable, body: `{"status":"error","errorType":"timeout","error":"query timed out in expression evaluation"}`},
			want:   "timeout: query timed out in expression evaluation",
		},
		{
			name:   "text error page",
			answer: &answer{status: http.StatusBadGateway, body: "upstream down\n"},
			want:   "error querying global datasource 'prom': HTTP 502 Bad Gateway: upstream down",
		},
		{
			name:   "native histograms",
			answer: new(ok(`{"resultType":"vector","result":[{"metric":{},"histogram":[1767355200,{"count":"2","sum":"3","buckets":[[0,"0","1","2"]]}]}]}`)),
			want:   "use histogram_quantile(), histogram_count() or histogram_sum()",
		},
		{
			name:   "too much data",
			answer: &answer{status: http.StatusOK, body: large},
			want:   "aggregate it (sum by, topk) or narrow the label matchers",
		},
		{
			name:      "wrong datasource kind",
			arguments: map[string]any{"query": "up", "datasource": map[string]any{"name": "logs"}},
			want:      "global datasource 'logs' is a LokiDatasource, not a PrometheusDatasource",
		},
		{
			name:      "unknown datasource",
			arguments: map[string]any{"query": "up", "datasource": map[string]any{"name": "nope"}},
			want:      "error retrieving global datasource 'nope' (set datasource.project for a project datasource)",
		},
		{
			name:      "time range of an instant query",
			arguments: map[string]any{"query": "up", "time_range": map[string]any{"past_duration": "6h"}},
			want:      "time_range and step apply to range queries",
		},
		{
			name:      "time of a range query",
			arguments: map[string]any{"query": "up", "type": "range", "time": "2026-01-02T10:00:00Z"},
			want:      "time applies to instant queries",
		},
		{
			name:      "invalid past duration",
			arguments: map[string]any{"query": "up", "type": "range", "time_range": map[string]any{"past_duration": "0s"}},
			want:      "invalid time_range.past_duration '0s'",
		},
		{
			name:      "invalid start",
			arguments: map[string]any{"query": "up", "type": "range", "time_range": map[string]any{"start": "yesterday"}},
			want:      "invalid time_range.start 'yesterday'",
		},
		{
			name:      "invalid step",
			arguments: map[string]any{"query": "up", "type": "range", "step": "fast"},
			want:      "invalid step 'fast'",
		},
		{
			name:      "invalid time zone",
			arguments: map[string]any{"query": "up", "time_zone": "Mars/Olympus"},
			want:      "invalid time_zone 'Mars/Olympus'",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			answers := map[string]answer{}
			if tt.answer != nil {
				answers[globalProxy+"/api/v1/query"] = *tt.answer
			}
			arguments := tt.arguments
			if arguments == nil {
				arguments = map[string]any{"query": "rate(x"}
			}
			text := call(t, connect(t, newFakePerses(t, answers)), QueryToolName, arguments, true)
			if !strings.Contains(text, tt.want) {
				t.Errorf("error %q does not contain %q", text, tt.want)
			}
		})
	}
}

func TestQueryFallsBackToGet(t *testing.T) {
	perses := newFakePerses(t, map[string]answer{
		"POST " + globalProxy + "/api/v1/query": {status: http.StatusMethodNotAllowed},
		globalProxy + "/api/v1/query":           ok(`{"resultType":"scalar","result":[1767355200,"1"]}`),
	})
	output := decode(t, call(t, connect(t, perses), QueryToolName, map[string]any{"query": "scalar(up)"}, false))
	if output["value"] != 1.0 {
		t.Errorf("output = %v", output)
	}
	if got := perses.Count(http.MethodGet, globalProxy+"/api/v1/query"); got != 1 {
		t.Errorf("%d GET requests, want 1", got)
	}
}

func TestQueryRangeKeepsTheHighestSeries(t *testing.T) {
	perses := newFakePerses(t, map[string]answer{
		globalProxy + "/api/v1/query_range": ok(`{"resultType":"matrix","result":[` +
			`{"metric":{"job":"a"},"values":[[1767355200,"1.00001"]]},` +
			`{"metric":{"job":"b"},"values":[[1767355200,"1.00004"]]}]}`),
	})
	output := decode(t, call(t, connect(t, perses), QueryToolName, map[string]any{"query": "up", "type": "range", "limit": 1}, false))
	if got := output["series"].([]any)[0].(map[string]any)["labels"]; !reflect.DeepEqual(got, map[string]any{"job": "b"}) {
		t.Errorf("returned series = %v, want job b (highest unrounded max)", got)
	}
}

func TestQueryInvalidTypeIsRejected(t *testing.T) {
	call(t, connect(t, newFakePerses(t, nil)), QueryToolName, map[string]any{"query": "up", "type": "bogus"}, true)
}

func TestDatasourceSettingsApplyToEveryRequest(t *testing.T) {
	perses := newFakePerses(t, map[string]answer{
		globalProxy + "/api/v1/labels":                ok(`["job"]`),
		globalProxy + "/api/v1/label/job/values":      ok(`["node"]`),
		globalProxy + "/api/v1/label/__name__/values": ok(`["up"]`),
		globalProxy + "/api/v1/query":                 ok(`{"resultType":"vector","result":[]}`),
		globalProxy + "/api/v1/query_range":           ok(`{"resultType":"matrix","result":[]}`),
	})
	perses.metadata = map[string][]v1.Metadata{"up": {{Type: v1.MetricTypeGauge}}}
	perses.datasources = []testDatasource{{
		Name: "prom", Kind: prometheusDatasourceKind, Default: true, ScrapeInterval: "30s",
		QueryParams: map[string]string{"dedup": "true", "start": "0", "tenant": "$tenant"},
	}}
	session := connect(t, perses)
	calls := []struct {
		tool      string
		arguments map[string]any
	}{
		{ListLabelNamesToolName, nil},
		{ListLabelValuesToolName, map[string]any{"label": "job"}},
		{ListMetricNamesToolName, map[string]any{"include_metadata": true}},
		{GetMetricMetadataToolName, map[string]any{"metrics": []any{"up"}}},
		{QueryToolName, map[string]any{"query": "up"}},
		{QueryToolName, map[string]any{"query": "up", "type": "range"}},
	}
	for _, c := range calls {
		output := decode(t, call(t, session, c.tool, c.arguments, false))
		// The parameter with a variable can't be resolved: the result says so.
		warnings, _ := output["warnings"].([]any)
		if len(warnings) == 0 || !strings.Contains(warnings[0].(string), "the query parameter 'tenant'") {
			t.Errorf("%s: warnings = %v", c.tool, output["warnings"])
		}
	}
	for _, request := range perses.Received() {
		if !strings.HasPrefix(request.Path, "/proxy/") {
			continue
		}
		query, err := url.ParseQuery(request.RawQuery)
		if err != nil {
			t.Fatal(err)
		}
		if query.Get("dedup") != "true" || query.Get("tenant") != "$tenant" {
			t.Errorf("%s %s: query = %v, want the query parameters of the datasource", request.Method, request.Path, query)
		}
	}
	// A parameter set by the request is kept: the GET request of the label values has its own start.
	if got := perses.paramsOf(globalProxy + "/api/v1/label/job/values")["start"]; !reflect.DeepEqual(got, []string{"1767351600"}) {
		t.Errorf("start = %v, want the start of the request only", got)
	}
	// The default step of a range query is at least the scrape interval of the datasource (1h / 500 < 30s).
	if got := perses.paramsOf(globalProxy + "/api/v1/query_range").Get("step"); got != "30" {
		t.Errorf("step = %s, want 30", got)
	}
}

func TestListTooLarge(t *testing.T) {
	path := globalProxy + "/api/v1/label/__name__/values"
	perses := newFakePerses(t, nil)
	perses.answer = func(r *http.Request) *answer {
		if r.URL.Path != path {
			return nil
		}
		// Prometheus filters the names when the search is sent as a matcher.
		if r.URL.Query().Get("match[]") == `{__name__=~"(?is).*MEMORY.*"}` {
			return new(ok(`["process_resident_memory_bytes","up_memory"]`))
		}
		return &answer{status: http.StatusOK, body: tooLarge(`[]`)}
	}
	session := connect(t, perses)

	// With search and without match, the list is requested again with search as a matcher.
	output := decode(t, call(t, session, ListMetricNamesToolName, map[string]any{"search": "MEMORY"}, false))
	if !reflect.DeepEqual(output["metrics"], []any{"process_resident_memory_bytes", "up_memory"}) {
		t.Errorf("metrics = %v", output["metrics"])
	}
	if got := perses.Count(http.MethodGet, path); got != 2 {
		t.Errorf("%d requests, want 2", got)
	}

	// Otherwise, the error tells how to narrow the list, not how to aggregate a query.
	for _, arguments := range []map[string]any{nil, {"search": "memory", "match": "up"}} {
		text := call(t, session, ListMetricNamesToolName, arguments, true)
		if !strings.Contains(text, errListTooLarge.Error()) {
			t.Errorf("error = %s, want %s", text, errListTooLarge)
		}
	}
}

func TestQueryParamsDontOverrideTheRequestParameters(t *testing.T) {
	labels := globalProxy + "/api/v1/labels"
	for _, fallback := range []bool{false, true} {
		t.Run(fmt.Sprintf("GET fallback %v", fallback), func(t *testing.T) {
			answers := map[string]answer{labels: ok(`["job"]`), globalProxy + "/api/v1/query_range": ok(`{"resultType":"matrix","result":[]}`)}
			if fallback {
				answers["POST "+labels] = answer{status: http.StatusMethodNotAllowed}
			}
			perses := newFakePerses(t, answers)
			perses.datasources = []testDatasource{{
				Name: "prom", Kind: prometheusDatasourceKind, Default: true,
				QueryParams: map[string]string{"match[]": "up", "start": "0", "dedup": "true"},
			}}
			session := connect(t, perses)
			call(t, session, ListLabelNamesToolName, map[string]any{"match": `{job="node"}`}, false)
			call(t, session, QueryToolName, map[string]any{"query": "up", "type": "range"}, false)

			// Prometheus merges a repeated parameter from the URL and the form body: the request has its own match[].
			method := http.MethodPost
			if fallback {
				method = http.MethodGet
			}
			params := effectiveParams(t, perses, method, labels)
			if !reflect.DeepEqual(params["match[]"], []string{`{job="node"}`}) {
				t.Errorf("match[] = %v, want the matcher of the request only", params["match[]"])
			}
			if params.Get("dedup") != "true" {
				t.Errorf("dedup = %q, want the query parameter of the datasource", params.Get("dedup"))
			}
			if got := effectiveParams(t, perses, http.MethodPost, globalProxy+"/api/v1/query_range")["start"]; !reflect.DeepEqual(got, []string{"1767351600"}) {
				t.Errorf("start = %v, want the start of the request only", got)
			}
		})
	}
}

// effectiveParams returns the parameters that Prometheus reads from the last request of method and path: the form
// body first, then the URL, as with http.Request.ParseForm.
func effectiveParams(t *testing.T, perses *fakePerses, method, path string) url.Values {
	t.Helper()
	received := perses.Received()
	for i := len(received) - 1; i >= 0; i-- {
		request := received[i]
		if request.Method != method || request.Path != path {
			continue
		}
		params := url.Values{}
		if request.ContentType == "application/x-www-form-urlencoded" {
			form, err := url.ParseQuery(request.Body)
			if err != nil {
				t.Fatal(err)
			}
			params = form
		}
		query, err := url.ParseQuery(request.RawQuery)
		if err != nil {
			t.Fatal(err)
		}
		for key, values := range query {
			params[key] = append(params[key], values...)
		}
		return params
	}
	t.Fatalf("no request %s %s", method, path)
	return nil
}
