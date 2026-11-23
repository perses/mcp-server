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
	"context"
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/perses/mcp-server/pkg/mcpapp"
	"github.com/perses/mcp-server/pkg/tools/internal/tooltest"
	"github.com/prometheus/common/model"
)

const queryRangePath = globalProxy + "/api/v1/query_range"

// testPanelsView stands for the view of perses_render_panels, which is not built for the tests.
const testPanelsView = "<html><body>panels</body></html>"

// newTestPanels returns the panels resource backed by perses, with the clock at testNow.
func newTestPanels(t *testing.T, perses http.Handler) *panels {
	t.Helper()
	p := NewPanels(tooltest.NewClient(t, perses), testPanelsView).(*panels)
	p.now = func() time.Time { return testNow }
	p.installed.now = p.now
	return p
}

// connectPanels connects a client to the panels tools. appHost makes the client declare MCP Apps.
func connectPanels(t *testing.T, perses http.Handler, appHost bool) *mcp.ClientSession {
	t.Helper()
	options := &mcp.ClientOptions{Capabilities: &mcp.ClientCapabilities{}}
	if appHost {
		options.Capabilities.AddExtension(mcpapp.ExtensionID, map[string]any{"mimeTypes": []any{mcpapp.ResourceMIMEType}})
	}
	return tooltest.ConnectClient(t, newTestPanels(t, perses).GetTools(), options, mcpapp.HideAppOnlyTools)
}

// answerQueries answers the range queries by their PromQL query.
func answerQueries(answers map[string]answer) func(r *http.Request) *answer {
	return func(r *http.Request) *answer {
		if !strings.HasSuffix(r.URL.Path, "/api/v1/query_range") {
			return nil
		}
		if err := r.ParseForm(); err != nil {
			return &answer{status: http.StatusBadRequest, body: err.Error()}
		}
		a, found := answers[r.Form.Get("query")]
		if !found {
			return &answer{status: http.StatusBadRequest, body: `{"status":"error","errorType":"bad_data","error":"unexpected query"}`}
		}
		return &a
	}
}

func matrix(series ...string) answer {
	return ok(`{"resultType":"matrix","result":[` + strings.Join(series, ",") + `]}`)
}

func callPanels(t *testing.T, session *mcp.ClientSession, arguments map[string]any, wantError bool) (*mcp.CallToolResult, string) {
	t.Helper()
	result, text := tooltest.CallTool(t, session, RenderPanelsToolName, arguments)
	if result.IsError != wantError {
		t.Fatalf("isError = %v, want %v: %s", result.IsError, wantError, text)
	}
	return result, text
}

func structured(t *testing.T, result *mcp.CallToolResult) map[string]any {
	t.Helper()
	data, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	return decode(t, string(data))
}

func TestRenderPanels(t *testing.T) {
	perses := newFakePerses(t, nil)
	perses.answer = answerQueries(map[string]answer{
		"heap": matrix(
			`{"metric":{"instance":"a"},"values":[[1767355140,"1"],[1767355200,"3"]]}`,
			`{"metric":{"instance":"b"},"values":[[1767355140,"5"],[1767355200,"4"]]}`,
		),
		"up": matrix(`{"metric":{"__name__":"up","job":"node"},"values":[[1767355200,"1"]]}`),
	})
	session := connectPanels(t, perses, false)
	result, text := callPanels(t, session, map[string]any{
		"time_range": map[string]any{"past_duration": "1h"},
		"panels": []any{
			map[string]any{
				"title": "Heap", "unit": "bytes",
				"queries": []any{map[string]any{"promql": "heap", "legend": "heap {{ instance }}"}},
				"spec":    map[string]any{"visual": map[string]any{"display": "bar"}},
			},
			map[string]any{"title": "Up", "kind": "StatChart", "queries": []any{map[string]any{"promql": "up"}}},
		},
	}, false)

	output := structured(t, result)
	if output["status"] != panelsStatusOK || output["step"] != "1m" || output["scrapeInterval"] != "1m" {
		t.Errorf("status = %v, step = %v, scrapeInterval = %v: %s", output["status"], output["step"], output["scrapeInterval"], text)
	}
	gotPanels, err := json.Marshal(output["panels"])
	if err != nil {
		t.Fatal(err)
	}
	wantPanels := `[{"kind":"Panel","spec":{"display":{"name":"Heap"},"plugin":{"kind":"TimeSeriesChart","spec":{"legend":{"mode":"list","position":"bottom"},` +
		`"visual":{"areaOpacity":0.1,"display":"bar","lineWidth":1.25},"yAxis":{"format":{"unit":"bytes"}}}},"queries":[{"kind":"TimeSeriesQuery","spec":{"plugin":` +
		`{"kind":"PrometheusTimeSeriesQuery","spec":{"datasource":{"kind":"PrometheusDatasource","name":"prom"},"query":"heap","seriesNameFormat":"heap {{ instance }}"}}}}]}},` +
		`{"kind":"Panel","spec":{"display":{"name":"Up"},"plugin":{"kind":"StatChart","spec":{"calculation":"last-number","sparkline":{}}},"queries":[{"kind":"TimeSeriesQuery",` +
		`"spec":{"plugin":{"kind":"PrometheusTimeSeriesQuery","spec":{"datasource":{"kind":"PrometheusDatasource","name":"prom"},"query":"up"}}}}]}}]`
	if string(gotPanels) != wantPanels {
		t.Errorf("panels =\n%s\nwant\n%s", gotPanels, wantPanels)
	}

	// The model gets the statistics, not the panel definitions.
	textOutput := decode(t, text)
	if _, found := textOutput["panels"]; found {
		t.Errorf("the text has the panels: %s", text)
	}
	if textOutput["panelCount"] != float64(2) {
		t.Errorf("panelCount = %v", textOutput["panelCount"])
	}
	gotSummaries, err := json.Marshal(textOutput["summaries"])
	if err != nil {
		t.Fatal(err)
	}
	wantSummaries := `[{"panel":"Heap","panelIndex":0,"promql":"heap","series":[` +
		`{"avg":4.5,"last":4,"max":5,"maxAt":"2026-01-02T11:59:00Z","min":4,"minAt":"2026-01-02T12:00:00Z","name":"heap b"},` +
		`{"avg":2,"last":3,"max":3,"maxAt":"2026-01-02T12:00:00Z","min":1,"minAt":"2026-01-02T11:59:00Z","name":"heap a"}],"seriesCount":2},` +
		`{"panel":"Up","panelIndex":1,"promql":"up","series":[{"avg":1,"last":1,"max":1,"maxAt":"2026-01-02T12:00:00Z","min":1,"minAt":"2026-01-02T12:00:00Z","name":"up{job=\"node\"}"}],"seriesCount":1}]`
	if string(gotSummaries) != wantSummaries {
		t.Errorf("summaries =\n%s\nwant\n%s", gotSummaries, wantSummaries)
	}

	// The panels are validated by Perses before the queries run.
	validations := perses.Count(http.MethodPost, "/api/validate/dashboards")
	if validations != 1 {
		t.Errorf("%d validation requests, want 1", validations)
	}
	for _, request := range perses.Received() {
		if request.Path == "/api/validate/dashboards" && (!strings.Contains(request.Body, `"mcp_panel_1":{"kind":"Panel"`) || request.Authorization != tooltest.Authorization) {
			t.Errorf("validated dashboard = %s", request.Body)
		}
	}
	params := perses.paramsOf(queryRangePath)
	if params.Get("step") != "60" || params.Get("start") != "1767351600" || params.Get("end") != "1767355200" {
		t.Errorf("query parameters = %v", params)
	}
}

func TestRenderPanelsWithProblems(t *testing.T) {
	perses := newFakePerses(t, nil)
	perses.answer = answerQueries(map[string]answer{
		"empty": matrix(),
		"bad":   {status: http.StatusBadRequest, body: `{"status":"error","errorType":"bad_data","error":"parse error"}`},
		"up":    matrix(`{"metric":{"job":"node"},"values":[[1767355200,"1"]]}`),
	})
	session := connectPanels(t, perses, false)
	panelInputs := []any{
		map[string]any{"title": "Empty", "queries": []any{map[string]any{"promql": "empty"}}},
		map[string]any{"title": "Up", "queries": []any{map[string]any{"promql": "up"}}},
	}

	result, text := callPanels(t, session, map[string]any{"panels": panelInputs}, false)
	output := structured(t, result)
	if output["status"] != panelsStatusError || len(output["panels"].([]any)) != 0 {
		t.Errorf("output = %s", text)
	}
	if message, _ := output["message"].(string); !strings.Contains(message, `panels[0] ("Empty") has no data`) || strings.Contains(message, "panels[1]") {
		t.Errorf("message = %q", message)
	}

	// allow_no_data shows the panels without data.
	result, text = callPanels(t, session, map[string]any{"panels": panelInputs, "allow_no_data": true}, false)
	if output = structured(t, result); output["status"] != panelsStatusOK || len(output["panels"].([]any)) != 2 {
		t.Errorf("with allow_no_data: %s", text)
	}

	// A failed query is always an error, with the error of Prometheus.
	panelInputs = append(panelInputs, map[string]any{"title": "Bad", "queries": []any{map[string]any{"promql": "bad"}}})
	result, text = callPanels(t, session, map[string]any{"panels": panelInputs, "allow_no_data": true}, false)
	output = structured(t, result)
	if message, _ := output["message"].(string); output["status"] != panelsStatusError || !strings.Contains(message, "Query `bad` of panels[2] failed: bad_data: parse error.") {
		t.Errorf("output = %s", text)
	}
}

func TestRenderPanelsWithTheScrapeInterval(t *testing.T) {
	perses := newFakePerses(t, nil)
	perses.datasources = []testDatasource{{Name: "prom", Kind: prometheusDatasourceKind, Default: true, ScrapeInterval: "15s"}}
	perses.answer = answerQueries(map[string]answer{"up": matrix(`{"metric":{"job":"node"},"values":[[1767355200,"1"]]}`)})
	session := connectPanels(t, perses, false)
	result, text := callPanels(t, session, map[string]any{
		"time_range": map[string]any{"past_duration": "5m"},
		"panels":     []any{map[string]any{"title": "Up", "queries": []any{map[string]any{"promql": "up"}}}},
	}, false)

	// The view gets the scrape interval of the datasource, which its Prometheus plugin uses as the minimum step.
	output := structured(t, result)
	if output["scrapeInterval"] != "15s" || output["step"] != "15s" {
		t.Errorf("scrapeInterval = %v, step = %v: %s", output["scrapeInterval"], output["step"], text)
	}
}

func TestRenderPanelsWithBuiltinVariables(t *testing.T) {
	perses := newFakePerses(t, nil)
	perses.datasources = []testDatasource{{Name: "prom", Kind: prometheusDatasourceKind, Default: true, ScrapeInterval: "15s"}}
	const promql = "rate(c[$__rate_interval]) + c offset ${__interval} + $__interval_ms"
	// Step 15s and min step 15s: $__rate_interval is max(15s + 15s, 4 * 15s).
	perses.answer = answerQueries(map[string]answer{
		"rate(c[1m]) + c offset 15s + 15000": matrix(`{"metric":{"job":"node"},"values":[[1767355200,"1"]]}`),
	})
	session := connectPanels(t, perses, false)
	result, text := callPanels(t, session, map[string]any{
		"time_range": map[string]any{"past_duration": "5m"},
		"panels":     []any{map[string]any{"title": "C", "queries": []any{map[string]any{"promql": promql}}}},
	}, false)

	// The statistics use the replaced query; the panel keeps the variables, which the view replaces.
	output := structured(t, result)
	summary := output["summaries"].([]any)[0].(map[string]any)
	if output["status"] != panelsStatusOK || summary["promql"] != promql || summary["seriesCount"] != float64(1) {
		t.Errorf("output = %s", text)
	}
	panels, err := json.Marshal(output["panels"])
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(panels), `"query":"rate(c[$__rate_interval]) + c offset ${__interval} + $__interval_ms"`) {
		t.Errorf("panels = %s", panels)
	}
}

func TestReplaceBuiltinVariables(t *testing.T) {
	for _, test := range []struct{ promql, want string }{
		{"up", "up"},
		{"rate(c[$__rate_interval])", "rate(c[4m])"},
		{"rate(c[${__rate_interval}])", "rate(c[4m])"},
		{"avg_over_time(g[$__interval]) * $__interval_ms", "avg_over_time(g[2m]) * 120000"},
		{"up{job=\"$job\"}", "up{job=\"$job\"}"},
	} {
		if got := replaceBuiltinVariables(test.promql, 2*time.Minute, time.Minute); got != test.want {
			t.Errorf("replaceBuiltinVariables(%q) = %q, want %q", test.promql, got, test.want)
		}
	}
	// $__rate_interval is $__interval + min step when the step is over 3 times the min step.
	if got := replaceBuiltinVariables("$__rate_interval", 5*time.Minute, time.Minute); got != "6m" {
		t.Errorf("$__rate_interval = %q, want 6m", got)
	}
}

func TestRenderPanelsWithNonFiniteSeries(t *testing.T) {
	perses := newFakePerses(t, nil)
	perses.answer = answerQueries(map[string]answer{"ratio": matrix(
		`{"metric":{"instance":"a"},"values":[[1767355140,"NaN"],[1767355200,"+Inf"]]}`,
		`{"metric":{"instance":"b"},"values":[[1767355140,"1"],[1767355200,"NaN"]]}`,
	)})
	session := connectPanels(t, perses, false)
	result, text := callPanels(t, session, map[string]any{
		"panels": []any{map[string]any{"title": "Ratio", "queries": []any{map[string]any{"promql": "ratio"}}}},
	}, false)

	// The series without statistics are counted, with a warning so that the model knows why they are missing.
	output := structured(t, result)
	summary := output["summaries"].([]any)[0].(map[string]any)
	if output["status"] != panelsStatusOK || summary["seriesCount"] != float64(2) || len(summary["series"].([]any)) != 1 {
		t.Errorf("output = %s", text)
	}
	want := "query `ratio` of panels[0]: 1 series have no statistics: they only have NaN or infinite samples in the time range"
	if warnings, _ := output["warnings"].([]any); !slices.Contains(warnings, any(want)) {
		t.Errorf("warnings = %v, want %q", output["warnings"], want)
	}
}

func TestRenderHistogramPanels(t *testing.T) {
	perses := newFakePerses(t, nil)
	perses.answer = answerQueries(map[string]answer{
		"classic": matrix(`{"metric":{"le":"0.1"},"values":[[1767355200,"1"]]}`),
		"native":  matrix(`{"metric":{},"histograms":[[1767355200,{"count":"2","sum":"1","buckets":[[0,"0","1","2"]]}]]}`),
	})
	session := connectPanels(t, perses, false)
	histogramPanel := func(query string) map[string]any {
		return map[string]any{"title": "Latency", "kind": "HeatMapChart", "unit": "seconds", "queries": []any{map[string]any{"promql": query}}}
	}

	result, text := callPanels(t, session, map[string]any{"panels": []any{histogramPanel("classic")}}, false)
	output := structured(t, result)
	if message, _ := output["message"].(string); output["status"] != panelsStatusError || !strings.Contains(message, "displays native histograms only") {
		t.Errorf("classic histogram: %s", text)
	}

	result, text = callPanels(t, session, map[string]any{"panels": []any{histogramPanel("native")}}, false)
	output = structured(t, result)
	if output["status"] != panelsStatusOK || !strings.Contains(text, `"histogramSeries":1`) {
		t.Errorf("native histogram: %s", text)
	}
}

func TestRenderPanelsInvalidInput(t *testing.T) {
	tests := []struct {
		name   string
		perses func(*fakePerses)
		panel  map[string]any
		want   string
	}{
		{
			name:  "unknown unit",
			panel: map[string]any{"title": "A", "unit": "parsecs", "queries": []any{map[string]any{"promql": "up"}}},
			want:  "unit",
		},
		{
			name:  "unit of a panel without unit",
			panel: map[string]any{"title": "A", "kind": "Table", "unit": "bytes", "queries": []any{map[string]any{"promql": "up"}}},
			want:  `panels[0] ("A"): Table panels have no unit: remove unit`,
		},
		{
			name:   "panel plugin not installed",
			perses: func(f *fakePerses) { f.panelPlugins = []string{"TimeSeriesChart", "Table"} },
			panel:  map[string]any{"title": "A", "kind": "PieChart", "queries": []any{map[string]any{"promql": "up"}}},
			want:   "the panel plugin 'PieChart' is not installed in Perses: use one of Table, TimeSeriesChart",
		},
		{
			name: "spec rejected by Perses",
			perses: func(f *fakePerses) {
				f.invalidDashboard = `panel "mcp_panel_0": invalid spec: visual.display: 3 errors`
			},
			panel: map[string]any{"title": "A", "spec": map[string]any{"visual": map[string]any{"display": "pie"}}, "queries": []any{map[string]any{"promql": "up"}}},
			want:  `invalid panels (fix spec, unit or kind): panel "panels[0]": invalid spec: visual.display: 3 errors`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			perses := newFakePerses(t, nil)
			if tt.perses != nil {
				tt.perses(perses)
			}
			session := connectPanels(t, perses, false)
			_, text := callPanels(t, session, map[string]any{"panels": []any{tt.panel}}, true)
			if !strings.Contains(text, tt.want) {
				t.Errorf("error = %q, want %q", text, tt.want)
			}
			if count := perses.Count(http.MethodPost, queryRangePath); count != 0 {
				t.Errorf("%d queries were run", count)
			}
		})
	}
}

func TestRenderPanelsWithoutThePluginList(t *testing.T) {
	perses := newFakePerses(t, nil)
	perses.pluginsUnavailable = true
	perses.answer = answerQueries(map[string]answer{"up": matrix(`{"metric":{"job":"node"},"values":[[1767355200,"1"]]}`)})
	session := connectPanels(t, perses, false)
	result, text := callPanels(t, session, map[string]any{
		"panels": []any{map[string]any{"title": "Up", "kind": "PieChart", "queries": []any{map[string]any{"promql": "up"}}}},
	}, false)
	output := structured(t, result)
	if output["status"] != panelsStatusOK || !strings.Contains(text, "the plugins installed in Perses could not be listed") {
		t.Errorf("output = %s", text)
	}
}

func TestRenderPanelsWithoutLoadedPanelPlugins(t *testing.T) {
	perses := newFakePerses(t, nil)
	perses.panelPlugins = []string{}
	perses.answer = answerQueries(map[string]answer{"up": matrix(`{"metric":{"job":"node"},"values":[[1767355200,"1"]]}`)})
	session := connectPanels(t, perses, false)
	result, text := callPanels(t, session, map[string]any{
		"panels": []any{map[string]any{"title": "Up", "kind": "PieChart", "queries": []any{map[string]any{"promql": "up"}}}},
	}, false)

	// The kinds are not all refused: they are not checked, with a warning. The empty list is not cached.
	output := structured(t, result)
	if output["status"] != panelsStatusOK || !strings.Contains(text, "Perses lists no loaded panel plugin") {
		t.Errorf("output = %s", text)
	}
	callPanels(t, session, map[string]any{
		"panels": []any{map[string]any{"title": "Up", "queries": []any{map[string]any{"promql": "up"}}}},
	}, false)
	if count := perses.Count(http.MethodGet, "/api/v1/plugins"); count != 2 {
		t.Errorf("%d plugin list requests, want 2", count)
	}
}

func TestInstalledPanelsAreCached(t *testing.T) {
	perses := newFakePerses(t, nil)
	p := newTestPanels(t, perses)
	now := testNow
	p.installed.now = func() time.Time { return now }
	for range 2 {
		if _, err := p.installed.get(); err != nil {
			t.Fatal(err)
		}
	}
	now = now.Add(installedPanelsTTL)
	kinds, err := p.installed.get()
	if err != nil {
		t.Fatal(err)
	}
	if count := perses.Count(http.MethodGet, "/api/v1/plugins"); count != 2 {
		t.Errorf("%d plugin list requests, want 2", count)
	}
	if !kinds["HeatMapChart"] || kinds[prometheusDatasourceKind] {
		t.Errorf("installed kinds = %v", kinds)
	}
}

func TestRenderPanelsTool(t *testing.T) {
	session := connectPanels(t, newFakePerses(t, nil), false)
	toolList, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	// The app-only tool is not listed to a client without MCP Apps.
	if len(toolList.Tools) != 1 || toolList.Tools[0].Name != RenderPanelsToolName {
		t.Fatalf("tools = %v", toolList.Tools)
	}
	ui, _ := toolList.Tools[0].Meta["ui"].(map[string]any)
	if ui["resourceUri"] != PanelsViewURI {
		t.Errorf("_meta = %v", toolList.Tools[0].Meta)
	}

	resource, err := session.ReadResource(context.Background(), &mcp.ReadResourceParams{URI: PanelsViewURI})
	if err != nil {
		t.Fatal(err)
	}
	content := resource.Contents[0]
	if content.MIMEType != mcpapp.ResourceMIMEType || content.Text != testPanelsView {
		t.Errorf("resource = %s %.100s", content.MIMEType, content.Text)
	}
	if _, found := content.Meta["ui"].(map[string]any)["csp"]; !found {
		t.Errorf("resource _meta = %v", content.Meta)
	}
}

func TestPanelsWithoutTheView(t *testing.T) {
	if got := NewPanels(tooltest.NewClient(t, newFakePerses(t, nil)), "").GetTools(); len(got) != 0 {
		t.Errorf("%d tools without the view, want none", len(got))
	}
}

func TestQueryData(t *testing.T) {
	perses := newFakePerses(t, map[string]answer{
		queryRangePath: matrix(`{"metric":{"job":"node"},"values":[[1767355200,"1"]]}`),
		globalProxy + "/api/v1/query": {status: http.StatusOK, body: `{"status":"success","data":{"resultType":"vector","result":[` +
			`{"metric":{"job":"node"},"value":[1767355200,"1"]}]},"warnings":["careful"]}`},
	})
	session := connectPanels(t, perses, true)
	toolList, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(toolList.Tools) != 2 {
		t.Errorf("an MCP Apps host must list the app-only tool: %v", toolList.Tools)
	}

	tests := []struct {
		name      string
		arguments map[string]any
		want      string
		params    string
	}{
		{
			name:      "range query",
			arguments: map[string]any{"query": "up", "start": "1767351600", "end": "2026-01-02T12:00:00Z", "step": "15"},
			want:      `{"data":{"result":[{"metric":{"job":"node"},"values":[[1767355200,"1"]]}],"resultType":"matrix"},"status":"success"}`,
			params:    "end=1767355200&query=up&start=1767351600&step=15",
		},
		{
			name:      "instant query",
			arguments: map[string]any{"query": "up", "time": "1767355200.5"},
			want:      `{"data":{"result":[{"metric":{"job":"node"},"value":[1767355200,"1"]}],"resultType":"vector"},"status":"success","warnings":["careful"]}`,
			params:    "query=up&time=1767355200.5",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, text := tooltest.CallTool(t, session, QueryDataToolName, tt.arguments)
			if result.IsError {
				t.Fatalf("error: %s", text)
			}
			got, err := json.Marshal(result.StructuredContent)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tt.want {
				t.Errorf("structured content =\n%s\nwant\n%s", got, tt.want)
			}
			// The data is only in the structured content: the text content is a summary.
			if !strings.HasPrefix(text, "Prometheus ") || !strings.HasSuffix(strings.TrimSpace(text), " result in the structured content.") {
				t.Errorf("text content = %s", text)
			}
			path := queryRangePath
			if tt.arguments["time"] != nil {
				path = globalProxy + "/api/v1/query"
			}
			if params := perses.paramsOf(path).Encode(); params != tt.params {
				t.Errorf("parameters = %s, want %s", params, tt.params)
			}
		})
	}

	_, text := tooltest.CallTool(t, session, QueryDataToolName, map[string]any{"query": "up", "start": "1767351600"})
	if !strings.Contains(text, "end is required") {
		t.Errorf("error = %s", text)
	}

	instantQuery := map[string]any{"query": "up", "time": "1767355200"}
	perses.answers[globalProxy+"/api/v1/query"] = answer{status: http.StatusOK,
		body: `{"status":"success","data":{"resultType":"vector","result":[` + strings.Repeat(" ", maxBodyBytes) + `]}}`}
	result, text := tooltest.CallTool(t, session, QueryDataToolName, instantQuery)
	if !result.IsError || !strings.Contains(text, "too large to display (over 1 MiB)") {
		t.Errorf("a Prometheus response over the limit must be an error: %s", text)
	}

	maxQueryDataBytes = 50
	t.Cleanup(func() { maxQueryDataBytes = maxBodyBytes })
	result, text = tooltest.CallTool(t, session, QueryDataToolName, map[string]any{"query": "up", "start": "1767351600", "end": "1767355200", "step": "15"})
	if !result.IsError || !strings.Contains(text, "too large to display (1 series") {
		t.Errorf("a response over the limit must be an error: %s", text)
	}
}

func TestQueryDataIsHiddenFromOtherClients(t *testing.T) {
	perses := newFakePerses(t, map[string]answer{
		globalProxy + "/api/v1/query": ok(`{"resultType":"vector","result":[]}`),
	})
	session := connectPanels(t, perses, false)
	toolList, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range toolList.Tools {
		if tool.Name == QueryDataToolName {
			t.Error("the app-only tool is listed to a client without MCP Apps")
		}
	}
	// It can still be called: some hosts render the views without declaring MCP Apps.
	result, text := tooltest.CallTool(t, session, QueryDataToolName, map[string]any{"query": "up"})
	if result.IsError {
		t.Errorf("error: %s", text)
	}
}

func TestMergeSpec(t *testing.T) {
	spec := map[string]any{"legend": map[string]any{"position": "bottom", "mode": "list"}, "visual": map[string]any{"display": "line"}}
	mergeSpec(spec, map[string]any{"legend": map[string]any{"position": "right"}, "visual": "replaced", "yAxis": map[string]any{"min": 0}})
	got, err := json.Marshal(spec)
	if err != nil {
		t.Fatal(err)
	}
	if want := `{"legend":{"mode":"list","position":"right"},"visual":"replaced","yAxis":{"min":0}}`; string(got) != want {
		t.Errorf("merged = %s, want %s", got, want)
	}
}

func TestPieChartPalette(t *testing.T) {
	datasource := DatasourceRef{Name: "prom"}
	panel := buildPanel(PanelInput{Title: "Memory"}, "PieChart", datasource)
	if palette, _ := panel.Spec.Plugin.Spec.(map[string]any)["colorPalette"].([]any); len(palette) != 7 {
		t.Errorf("default palette = %v", palette)
	}
	panel = buildPanel(PanelInput{Title: "Memory", Spec: map[string]any{"colorPalette": []any{"#ff0000", "#00ff00"}}}, "PieChart", datasource)
	if palette, _ := panel.Spec.Plugin.Spec.(map[string]any)["colorPalette"].([]any); !slices.Equal(palette, []any{"#ff0000", "#00ff00"}) {
		t.Errorf("palette of the model = %v", palette)
	}
}

func TestSeriesName(t *testing.T) {
	metric := model.Metric{"__name__": "up", "job": "node", "instance": "a:9100"}
	tests := []struct {
		legend string
		want   string
	}{
		{legend: "", want: `up{instance="a:9100", job="node"}`},
		{legend: "{{job}} on {{ instance }}", want: "node on a:9100"},
		{legend: "{{missing}}x", want: "x"},
	}
	for _, tt := range tests {
		if got := seriesName(metric, tt.legend); got != tt.want {
			t.Errorf("seriesName(%q) = %q, want %q", tt.legend, got, tt.want)
		}
	}
	if got := seriesName(model.Metric{}, ""); got != "{}" {
		t.Errorf("seriesName of no label = %q", got)
	}
}

func TestParseAPITimeAndDuration(t *testing.T) {
	at, err := parseAPITime("time", "1767355200.25")
	if err != nil || !at.Equal(testNow.Add(250*time.Millisecond)) {
		t.Errorf("parseAPITime = %v, %v", at, err)
	}
	if _, err = parseAPITime("time", "yesterday"); err == nil {
		t.Error("parseAPITime accepts yesterday")
	}
	for value, want := range map[string]time.Duration{"15": 15 * time.Second, "0.5": 500 * time.Millisecond, "1m": time.Minute} {
		if got, durationErr := parseAPIDuration("step", value); durationErr != nil || got != want {
			t.Errorf("parseAPIDuration(%s) = %v, %v", value, got, durationErr)
		}
	}
	for _, value := range []string{"0", "-1", "NaN", "soon"} {
		if _, durationErr := parseAPIDuration("step", value); durationErr == nil {
			t.Errorf("parseAPIDuration accepts %s", value)
		}
	}
}
