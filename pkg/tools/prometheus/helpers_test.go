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
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/perses/mcp-server/pkg/tools/internal/tooltest"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
)

var testNow = time.Date(2026, 1, 2, 12, 0, 0, 0, time.UTC)

const (
	globalProxy  = "/proxy/globaldatasources/prom"
	projectProxy = "/proxy/projects/demo/datasources/prom"
)

// testDatasource is a datasource served by fakePerses. Project is empty for a global datasource.
type testDatasource struct {
	Name    string
	Project string
	Kind    string
	Default bool
}

// defaultDatasources are a global default Prometheus datasource "prom", a project Prometheus datasource "prom" in
// project "demo" and a global default Loki datasource "logs".
var defaultDatasources = []testDatasource{
	{Name: "prom", Kind: prometheusDatasourceKind, Default: true},
	{Name: "prom", Project: "demo", Kind: prometheusDatasourceKind},
	{Name: "logs", Kind: "LokiDatasource", Default: true},
}

type answer struct {
	status int
	body   string
}

func ok(data string) answer {
	return answer{status: http.StatusOK, body: `{"status":"success","data":` + data + `}`}
}

// fakePerses serves the datasource API of Perses for its datasources and answers the proxy requests by path, or by
// "METHOD path" first. The embedded tooltest.Perses records the requests.
type fakePerses struct {
	tooltest.Perses
	t           *testing.T
	datasources []testDatasource
	answers     map[string]answer
	// metadata, when set, is the metadata catalog served on /api/v1/metadata, filtered by the parameter metric.
	metadata map[string][]v1.Metadata
}

func newFakePerses(t *testing.T, answers map[string]answer) *fakePerses {
	f := &fakePerses{t: t, datasources: defaultDatasources, answers: answers}
	f.Handler = http.HandlerFunc(f.serve)
	return f
}

func (f *fakePerses) serve(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/v1/") {
		f.serveDatasources(w, r)
		return
	}
	if f.metadata != nil && strings.HasSuffix(r.URL.Path, "/api/v1/metadata") {
		f.serveMetadata(w, r.URL.Query().Get("metric"))
		return
	}
	a, found := f.answers[r.Method+" "+r.URL.Path]
	if !found {
		a, found = f.answers[r.URL.Path]
	}
	if !found {
		f.t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(a.status)
	_, _ = w.Write([]byte(a.body))
}

func (f *fakePerses) serveMetadata(w http.ResponseWriter, metric string) {
	metadata := f.metadata
	if metric != "" {
		metadata = map[string][]v1.Metadata{}
		if entries, found := f.metadata[metric]; found {
			metadata[metric] = entries
		}
	}
	data, err := json.Marshal(metadata)
	if err != nil {
		// The handler runs in the goroutine of the HTTP server, where t.Fatal must not be called.
		f.t.Errorf("encoding the metadata: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"status":"success","data":` + string(data) + `}`))
}

// paramsOf returns the parameters (query string and form) of the last request of path.
func (f *fakePerses) paramsOf(path string) url.Values {
	f.t.Helper()
	received := f.Received()
	for i := len(received) - 1; i >= 0; i-- {
		request := received[i]
		if request.Path != path {
			continue
		}
		params, err := url.ParseQuery(request.RawQuery)
		if err != nil {
			f.t.Fatalf("parsing the query of %s: %v", path, err)
		}
		if request.ContentType == "application/x-www-form-urlencoded" {
			form, formErr := url.ParseQuery(request.Body)
			if formErr != nil {
				f.t.Fatalf("parsing the form of %s: %v", path, formErr)
			}
			for key, values := range form {
				params[key] = append(params[key], values...)
			}
		}
		return params
	}
	f.t.Fatalf("no request received for %s", path)
	return nil
}

// serveDatasources serves the get and list requests of global and project datasources.
func (f *fakePerses) serveDatasources(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	var project, name string
	switch {
	case len(parts) >= 3 && len(parts) <= 4 && parts[2] == "globaldatasources":
		if len(parts) == 4 {
			name = parts[3]
		}
	case len(parts) >= 5 && len(parts) <= 6 && parts[2] == "projects" && parts[4] == "datasources":
		project = parts[3]
		if len(parts) == 6 {
			name = parts[5]
		}
	default:
		f.t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
		return
	}
	found := []any{}
	for _, ds := range f.datasources {
		if ds.Project == project && (name == "" || ds.Name == name) {
			found = append(found, encodeDatasource(ds))
		}
	}
	w.Header().Set("Content-Type", "application/json")
	if name == "" {
		_ = json.NewEncoder(w).Encode(found)
		return
	}
	if len(found) == 0 {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"document not found"}`))
		return
	}
	_ = json.NewEncoder(w).Encode(found[0])
}

func encodeDatasource(ds testDatasource) map[string]any {
	kind := "GlobalDatasource"
	metadata := map[string]any{"name": ds.Name}
	if ds.Project != "" {
		kind = "Datasource"
		metadata["project"] = ds.Project
	}
	return map[string]any{
		"kind":     kind,
		"metadata": metadata,
		"spec": map[string]any{
			"default": ds.Default,
			"plugin":  map[string]any{"kind": ds.Kind, "spec": map[string]any{}},
		},
	}
}

// newTestPrometheus returns the prometheus resource backed by perses, with the clock at testNow.
func newTestPrometheus(t *testing.T, perses http.Handler) *prometheus {
	t.Helper()
	p := New(tooltest.NewClient(t, perses).V1()).(*prometheus)
	p.now = func() time.Time { return testNow }
	return p
}

// connect registers the Prometheus tools on an in-memory MCP server backed by perses.
func connect(t *testing.T, perses http.Handler) *mcp.ClientSession {
	t.Helper()
	return tooltest.Connect(t, newTestPrometheus(t, perses).GetTools())
}

// call calls a tool and returns its text, failing when isError is not wantError.
func call(t *testing.T, session *mcp.ClientSession, name string, arguments map[string]any, wantError bool) string {
	t.Helper()
	result, text := tooltest.CallTool(t, session, name, arguments)
	if result.IsError != wantError {
		t.Fatalf("isError = %v, want %v: %s", result.IsError, wantError, text)
	}
	return text
}

func decode(t *testing.T, text string) map[string]any {
	t.Helper()
	var output map[string]any
	if err := json.Unmarshal([]byte(text), &output); err != nil {
		t.Fatalf("decoding %s: %v", text, err)
	}
	return output
}
