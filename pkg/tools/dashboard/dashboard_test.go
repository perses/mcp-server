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
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/perses/mcp-server/pkg/tools"
	"github.com/perses/mcp-server/pkg/tools/internal/dashboardutil"
	apiClient "github.com/perses/perses/pkg/client/api"
	"github.com/perses/perses/pkg/client/perseshttp"
	v1 "github.com/perses/perses/pkg/model/api/v1"
	"github.com/perses/spec/go/common"
	dashboardSpec "github.com/perses/spec/go/dashboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const validDashboard = `{
	"kind": "Dashboard",
	"metadata": {"name": "cpu"},
	"spec": {
		"duration": "1h",
		"panels": {"p1": {"kind": "Panel", "spec": {"display": {"name": "CPU"}, "plugin": {"kind": "StatChart", "spec": {"calculation": "mean"}}}}},
		"layouts": [{"kind": "Grid", "spec": {"display": {"title": "Row"}, "items": [{"x": 0, "y": 0, "width": 12, "height": 6, "content": {"$ref": "#/spec/panels/p1"}}]}}]
	}
}`

const statPanel = `{"kind": "Panel", "spec": {"display": {"name": "%s"}, "plugin": {"kind": "StatChart", "spec": {"calculation": "%s"}}}}`

// fakePerses is an in-memory Perses API storing dashboards as raw JSON documents, with a validation endpoint
// rejecting the calculation "average" like the real schemas do.
type fakePerses struct {
	mu         sync.Mutex
	dashboards map[string]map[string]any
	getStatus  int
	// requests counts the requests per "METHOD path".
	requests map[string]int
}

func newFakePerses() *fakePerses {
	return &fakePerses{dashboards: map[string]map[string]any{}, requests: map[string]int{}}
}

func (f *fakePerses) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.requests[r.Method+" "+r.URL.Path]++
	body, _ := io.ReadAll(r.Body)

	if r.Method == http.MethodPost && r.URL.Path == "/api/validate/dashboards" {
		if strings.Contains(string(body), `"average"`) {
			f.writeMessage(w, http.StatusBadRequest, "bad request: bad request: invalid panel p2: spec.calculation: conflicting values \"mean\" and \"average\":\n    1:1\n")
			return
		}
		w.WriteHeader(http.StatusOK)
		return
	}

	const prefix = "/api/v1/projects/myproject/dashboards"
	if !strings.HasPrefix(r.URL.Path, prefix) {
		f.writeMessage(w, http.StatusNotFound, "not found")
		return
	}
	name := strings.TrimPrefix(strings.TrimPrefix(r.URL.Path, prefix), "/")
	switch {
	case r.Method == http.MethodGet && name == "":
		list := make([]map[string]any, 0, len(f.dashboards))
		for _, dash := range f.dashboards {
			list = append(list, dash)
		}
		f.writeJSON(w, http.StatusOK, list)
	case r.Method == http.MethodGet:
		if f.getStatus != 0 {
			f.writeMessage(w, f.getStatus, "dashboard lookup failed")
			return
		}
		dash, ok := f.dashboards[name]
		if !ok {
			f.writeMessage(w, http.StatusNotFound, "document not found")
			return
		}
		f.writeJSON(w, http.StatusOK, dash)
	case r.Method == http.MethodPost:
		dash := decode(body)
		name = dash["metadata"].(map[string]any)["name"].(string)
		if _, exists := f.dashboards[name]; exists {
			f.writeMessage(w, http.StatusConflict, "document already exists")
			return
		}
		f.store(dash, 0)
		f.writeJSON(w, http.StatusOK, dash)
	case r.Method == http.MethodPut:
		current, ok := f.dashboards[name]
		if !ok {
			f.writeMessage(w, http.StatusNotFound, "document not found")
			return
		}
		dash := decode(body)
		f.store(dash, uint64(current["metadata"].(map[string]any)["version"].(float64))+1)
		f.writeJSON(w, http.StatusOK, dash)
	case r.Method == http.MethodDelete:
		delete(f.dashboards, name)
		w.WriteHeader(http.StatusNoContent)
	default:
		f.writeMessage(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func decode(body []byte) map[string]any {
	var dash map[string]any
	_ = json.Unmarshal(body, &dash)
	return dash
}

func (f *fakePerses) store(dash map[string]any, version uint64) {
	metadata := dash["metadata"].(map[string]any)
	metadata["version"] = float64(version)
	metadata["project"] = "myproject"
	f.dashboards[metadata["name"].(string)] = dash
}

func (f *fakePerses) writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func (f *fakePerses) writeMessage(w http.ResponseWriter, status int, message string) {
	f.writeJSON(w, status, map[string]string{"message": message})
}

// stored returns the stored dashboard re-encoded as JSON, for assertions.
func (f *fakePerses) stored(t *testing.T) string {
	const name = "cpu"
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	dash, ok := f.dashboards[name]
	require.True(t, ok, "dashboard %s is not stored", name)
	text, err := json.Marshal(dash)
	require.NoError(t, err)
	return string(text)
}

func newSession(t *testing.T, fake *fakePerses) *mcp.ClientSession {
	t.Helper()
	srv := httptest.NewServer(fake)
	t.Cleanup(srv.Close)

	client := apiClient.NewWithClient(&perseshttp.RESTClient{
		BaseURL: common.MustParseURL(srv.URL),
		Client:  srv.Client(),
	})
	return connect(t, client)
}

func connect(t *testing.T, client apiClient.ClientInterface) *mcp.ClientSession {
	t.Helper()
	server := mcp.NewServer(&mcp.Implementation{Name: "perses-mcp-server-test"}, nil)
	for _, tool := range New(client).GetTools() {
		tool.RegisterWith(server)
	}

	ctx := context.Background()
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(ctx, serverTransport, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = serverSession.Close() })

	session, err := mcp.NewClient(&mcp.Implementation{Name: "test-client"}, nil).Connect(ctx, clientTransport, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = session.Close() })
	return session
}

// callTool calls a tool and returns its first text content and whether the result is an error.
func callTool(t *testing.T, session *mcp.ClientSession, name string, args map[string]any) (string, bool) {
	t.Helper()
	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	require.NoError(t, err, "protocol error calling %s", name)
	require.Len(t, result.Content, 1)
	require.Nil(t, result.StructuredContent, "the dashboard tools must not duplicate the payload as structured content")
	text, ok := result.Content[0].(*mcp.TextContent)
	require.True(t, ok, "expected text content, got %T", result.Content[0])
	return text.Text, result.IsError
}

func object(t *testing.T, text string) map[string]any {
	t.Helper()
	var value map[string]any
	require.NoError(t, json.Unmarshal([]byte(text), &value), "not a JSON object: %s", text)
	return value
}

func mustCall(t *testing.T, session *mcp.ClientSession, name string, args map[string]any) map[string]any {
	t.Helper()
	text, isError := callTool(t, session, name, args)
	require.False(t, isError, "unexpected tool error: %s", text)
	return object(t, text)
}

func mustFail(t *testing.T, session *mcp.ClientSession, name string, args map[string]any) string {
	t.Helper()
	text, isError := callTool(t, session, name, args)
	require.True(t, isError, "expected a tool error, got: %s", text)
	return text
}

func validationOf(t *testing.T, text string) dashboardutil.Validation {
	t.Helper()
	var validation dashboardutil.Validation
	require.NoError(t, json.Unmarshal([]byte(text), &validation), "not a validation: %s", text)
	return validation
}

func TestGetTools(t *testing.T) {
	names := []string{}
	for _, tool := range New(nil).GetTools() {
		names = append(names, tool.MCPTool.Name)
		assert.NotNil(t, tool.MCPTool.InputSchema, tool.MCPTool.Name)
	}
	assert.Equal(t, []string{
		"perses_list_dashboards", "perses_get_dashboard_by_name", "perses_create_dashboard", "perses_update_dashboard",
		"perses_delete_dashboard", "perses_set_dashboard_panels", "perses_set_dashboard_variables",
		"perses_set_dashboard_layout",
	}, names)
}

func TestCreate(t *testing.T) {
	var asObject map[string]any
	require.NoError(t, json.Unmarshal([]byte(validDashboard), &asObject))

	t.Run("object input returns a summary", func(t *testing.T) {
		fake := newFakePerses()
		session := newSession(t, fake)
		result := mustCall(t, session, "perses_create_dashboard", map[string]any{"project": "myproject", "dashboard": asObject})
		assert.Equal(t, "created", result["action"])
		assert.Equal(t, "cpu", result["name"])
		assert.Equal(t, "myproject", result["project"])
		assert.Equal(t, float64(1), result["panels"])
		assert.Equal(t, "/projects/myproject/dashboards/cpu", result["url"].(string)[strings.Index(result["url"].(string), "/projects"):])
		assert.Nil(t, result["dashboard"])
		assert.Equal(t, 1, fake.requests["POST /api/validate/dashboards"])
	})
	t.Run("string input is still accepted", func(t *testing.T) {
		session := newSession(t, newFakePerses())
		result := mustCall(t, session, "perses_create_dashboard", map[string]any{"project": "myproject", "dashboard": validDashboard})
		assert.Equal(t, "created", result["action"])
	})
	t.Run("include_dashboard", func(t *testing.T) {
		session := newSession(t, newFakePerses())
		result := mustCall(t, session, "perses_create_dashboard", map[string]any{"project": "myproject", "dashboard": asObject, "include_dashboard": true})
		assert.NotNil(t, result["dashboard"])
	})
	t.Run("dry run saves nothing", func(t *testing.T) {
		fake := newFakePerses()
		session := newSession(t, fake)
		result := mustCall(t, session, "perses_create_dashboard", map[string]any{"project": "myproject", "dashboard": asObject, "dry_run": true})
		assert.Equal(t, "would_create", result["action"])
		assert.Equal(t, true, result["dryRun"])
		assert.Empty(t, fake.dashboards)
	})
	t.Run("dry run reports dashboard lookup failures", func(t *testing.T) {
		fake := newFakePerses()
		fake.getStatus = http.StatusInternalServerError
		session := newSession(t, fake)
		text := mustFail(t, session, "perses_create_dashboard", map[string]any{"project": "myproject", "dashboard": asObject, "dry_run": true})
		assert.Contains(t, text, "error checking whether dashboard 'cpu' exists")
		assert.Empty(t, fake.dashboards)
		assert.Zero(t, fake.requests["POST /api/v1/projects/myproject/dashboards"])
	})
	t.Run("conflict", func(t *testing.T) {
		fake := newFakePerses()
		session := newSession(t, fake)
		mustCall(t, session, "perses_create_dashboard", map[string]any{"project": "myproject", "dashboard": asObject})
		text := mustFail(t, session, "perses_create_dashboard", map[string]any{"project": "myproject", "dashboard": asObject})
		assert.Contains(t, text, "already exists")
		assert.Contains(t, text, "perses_update_dashboard")

		text = mustFail(t, session, "perses_create_dashboard", map[string]any{"project": "myproject", "dashboard": asObject, "dry_run": true})
		assert.Contains(t, text, "already exists")
		assert.Contains(t, text, "perses_update_dashboard")
		assert.Equal(t, 2, fake.requests["POST /api/v1/projects/myproject/dashboards"], "the dry run must not save")
	})
	t.Run("validation error is structured", func(t *testing.T) {
		fake := newFakePerses()
		session := newSession(t, fake)
		invalid := strings.ReplaceAll(validDashboard, `p1`, `p2`)
		invalid = strings.Replace(invalid, `"mean"`, `"average"`, 1)
		text := mustFail(t, session, "perses_create_dashboard", map[string]any{"project": "myproject", "dashboard": invalid})
		validation := validationOf(t, text)
		require.Len(t, validation.Errors, 1)
		assert.Equal(t, "spec.panels.p2.spec.plugin.spec.calculation", validation.Errors[0].Path)
		assert.Equal(t, `conflicting values "mean" and "average"`, validation.Errors[0].Message)
		assert.Empty(t, fake.dashboards)
	})
	t.Run("decode error is structured", func(t *testing.T) {
		session := newSession(t, newFakePerses())
		text := mustFail(t, session, "perses_create_dashboard", map[string]any{"project": "myproject", "dashboard": `{"kind": "Dashboard"`})
		validation := validationOf(t, text)
		require.Len(t, validation.Errors, 1)
		assert.Contains(t, validation.Errors[0].Message, "malformed JSON")
	})
	t.Run("query without wrapper is located", func(t *testing.T) {
		fake := newFakePerses()
		session := newSession(t, fake)
		shortQuery := strings.Replace(validDashboard, `"spec": {"calculation": "mean"}}`,
			`"spec": {"calculation": "mean"}}, "queries": [{"kind": "PrometheusTimeSeriesQuery", "spec": {"query": "up"}}]`, 1)
		text := mustFail(t, session, "perses_create_dashboard", map[string]any{"project": "myproject", "dashboard": shortQuery, "dry_run": true})
		validation := validationOf(t, text)
		require.Len(t, validation.Errors, 1)
		assert.Equal(t, "spec.panels.p1.spec.queries[0]", validation.Errors[0].Path)
		assert.Contains(t, validation.Errors[0].Hint, `{kind: "TimeSeriesQuery", spec: {plugin: {kind: "PrometheusTimeSeriesQuery", spec: {query: ...}}}}`)
		assert.Zero(t, fake.requests["POST /api/validate/dashboards"])
	})
	t.Run("rejects trailing JSON values", func(t *testing.T) {
		fake := newFakePerses()
		session := newSession(t, fake)
		text := mustFail(t, session, "perses_create_dashboard", map[string]any{"project": "myproject", "dashboard": validDashboard + " {}"})
		validation := validationOf(t, text)
		require.Len(t, validation.Errors, 1)
		assert.Contains(t, validation.Errors[0].Message, "unexpected JSON value after dashboard")
		assert.Empty(t, fake.dashboards)
		assert.Zero(t, fake.requests["POST /api/validate/dashboards"])
	})
	t.Run("warnings are returned with the summary", func(t *testing.T) {
		session := newSession(t, newFakePerses())
		withOrphan := strings.Replace(validDashboard, `"panels": {`, `"panels": {"orphan": `+strings.ReplaceAll(statPanel, "%s", "x")+`, `, 1)
		result := mustCall(t, session, "perses_create_dashboard", map[string]any{"project": "myproject", "dashboard": withOrphan})
		warnings := result["warnings"].([]any)
		require.Len(t, warnings, 1)
		assert.Equal(t, "spec.panels.orphan", warnings[0].(map[string]any)["path"])
	})
}

func TestUpdate(t *testing.T) {
	fake := newFakePerses()
	session := newSession(t, fake)
	mustCall(t, session, "perses_create_dashboard", map[string]any{"project": "myproject", "dashboard": validDashboard})

	t.Run("missing dashboard", func(t *testing.T) {
		missing := strings.Replace(validDashboard, `"cpu"`, `"other"`, 1)
		for _, extra := range []map[string]any{{}, {"expected_version": 0}, {"dry_run": true}} {
			args := map[string]any{"project": "myproject", "dashboard": missing}
			for key, value := range extra {
				args[key] = value
			}
			text := mustFail(t, session, "perses_update_dashboard", args)
			assert.Equal(t, "dashboard 'other' does not exist in project 'myproject'", text, "%v", extra)
		}
		assert.NotContains(t, fake.dashboards, "other")
		assert.Equal(t, 1, fake.requests["PUT /api/v1/projects/myproject/dashboards/other"], "without a version check, the PUT reports the missing dashboard")
	})
	t.Run("update without expected version does not read", func(t *testing.T) {
		before := fake.requests["GET /api/v1/projects/myproject/dashboards/cpu"]
		result := mustCall(t, session, "perses_update_dashboard", map[string]any{"project": "myproject", "dashboard": validDashboard})
		assert.Equal(t, "updated", result["action"])
		assert.Equal(t, before, fake.requests["GET /api/v1/projects/myproject/dashboards/cpu"], "an update-only role must be able to update")
	})
	t.Run("version mismatch", func(t *testing.T) {
		text := mustFail(t, session, "perses_update_dashboard", map[string]any{"project": "myproject", "dashboard": validDashboard, "expected_version": 7})
		assert.Contains(t, text, "is at version 1, not 7")
	})
	t.Run("update", func(t *testing.T) {
		result := mustCall(t, session, "perses_update_dashboard", map[string]any{"project": "myproject", "dashboard": validDashboard, "expected_version": 1})
		assert.Equal(t, "updated", result["action"])
		assert.Equal(t, float64(2), result["version"])
	})
	t.Run("dry run saves nothing", func(t *testing.T) {
		fake := newFakePerses()
		session := newSession(t, fake)
		mustCall(t, session, "perses_create_dashboard", map[string]any{"project": "myproject", "dashboard": validDashboard})
		before, err := json.Marshal(fake.dashboards["cpu"])
		require.NoError(t, err)

		renamed := strings.Replace(validDashboard, `"name": "CPU"`, `"name": "Renamed"`, 1)
		result := mustCall(t, session, "perses_update_dashboard", map[string]any{"project": "myproject", "dashboard": renamed, "dry_run": true, "expected_version": 0})
		assert.Equal(t, "would_update", result["action"])
		assert.Equal(t, true, result["dryRun"])
		assert.Zero(t, fake.requests["PUT /api/v1/projects/myproject/dashboards/cpu"])
		after, err := json.Marshal(fake.dashboards["cpu"])
		require.NoError(t, err)
		assert.JSONEq(t, string(before), string(after))
		assert.Equal(t, float64(0), fake.dashboards["cpu"]["metadata"].(map[string]any)["version"])
	})
}

func TestSetPanels(t *testing.T) {
	fake := newFakePerses()
	session := newSession(t, fake)
	mustCall(t, session, "perses_create_dashboard", map[string]any{"project": "myproject", "dashboard": validDashboard})

	t.Run("add panels with automatic placement", func(t *testing.T) {
		panels := map[string]any{
			"p2": object(t, strings.ReplaceAll(statPanel, "%s", "P2")),
			"p3": object(t, strings.ReplaceAll(statPanel, "%s", "P3")),
		}
		result := mustCall(t, session, "perses_set_dashboard_panels", map[string]any{"project": "myproject", "name": "cpu", "panels": panels})
		assert.Equal(t, "updated", result["action"])
		assert.Equal(t, float64(3), result["panels"])
		changes := result["changes"].([]any)
		assert.Equal(t, "added panel p2, placed in layout 0 at x=12 y=0 w=12 h=6", changes[0])
		assert.Equal(t, "added panel p3, placed in layout 0 at x=0 y=6 w=12 h=6", changes[1])
		assert.Nil(t, result["warnings"])
		assert.Contains(t, fake.stored(t), `"$ref":"#/spec/panels/p3"`)
	})
	t.Run("replace keeps the position", func(t *testing.T) {
		panels := map[string]any{"p2": object(t, strings.ReplaceAll(statPanel, "%s", "P2 bis"))}
		result := mustCall(t, session, "perses_set_dashboard_panels", map[string]any{"project": "myproject", "name": "cpu", "panels": panels})
		assert.Equal(t, []any{"replaced panel p2, kept its position"}, result["changes"])
		assert.Equal(t, float64(3), result["panels"])
		assert.Contains(t, fake.stored(t), "P2 bis")
	})
	t.Run("new layout group and explicit placement", func(t *testing.T) {
		panels := map[string]any{
			"p4": object(t, strings.ReplaceAll(statPanel, "%s", "P4")),
			"p5": object(t, strings.ReplaceAll(statPanel, "%s", "P5")),
		}
		result := mustCall(t, session, "perses_set_dashboard_panels", map[string]any{
			"project": "myproject", "name": "cpu", "panels": panels, "layout": "Details", "width": 24, "height": 8,
			"placements": map[string]any{"p5": map[string]any{"x": 6, "y": 20, "width": 6}},
		})
		changes := result["changes"].([]any)
		assert.Equal(t, "added panel p4, placed in layout 1 at x=0 y=26 w=24 h=8", changes[0])
		assert.Equal(t, "added panel p5, placed in layout 1 at x=6 y=20 w=6 h=6", changes[1])
		assert.Equal(t, float64(2), result["layouts"])
	})
	t.Run("panel rejected by the schemas", func(t *testing.T) {
		panels := map[string]any{"p2": object(t, strings.Replace(strings.ReplaceAll(statPanel, "%s", "P2"), "P2\"}}", "average\"}}", 1))}
		text := mustFail(t, session, "perses_set_dashboard_panels", map[string]any{"project": "myproject", "name": "cpu", "panels": panels})
		validation := validationOf(t, text)
		require.Len(t, validation.Errors, 1)
		assert.Equal(t, "spec.panels.p2.spec.plugin.spec.calculation", validation.Errors[0].Path)
		assert.Contains(t, fake.stored(t), "P2 bis", "the dashboard must not be modified")
	})
	t.Run("query without wrapper is located", func(t *testing.T) {
		panel := strings.Replace(strings.ReplaceAll(statPanel, "%s", "P2"), `}}}}`,
			`}}, "queries": [{"kind": "PrometheusTimeSeriesQuery", "spec": {"query": "up"}}]}}`, 1)
		text := mustFail(t, session, "perses_set_dashboard_panels", map[string]any{"project": "myproject", "name": "cpu", "panels": map[string]any{"p2": object(t, panel)}})
		validation := validationOf(t, text)
		require.Len(t, validation.Errors, 1)
		assert.Equal(t, "panels.p2.spec.queries[0]", validation.Errors[0].Path)
		assert.Contains(t, validation.Errors[0].Hint, `{kind: "TimeSeriesQuery", spec: {plugin: {kind: "PrometheusTimeSeriesQuery"`)
		assert.Contains(t, fake.stored(t), "P2 bis", "the dashboard must not be modified")
	})
	t.Run("invalid key", func(t *testing.T) {
		panels := map[string]any{"bad key": object(t, strings.ReplaceAll(statPanel, "%s", "x"))}
		text := mustFail(t, session, "perses_set_dashboard_panels", map[string]any{"project": "myproject", "name": "cpu", "panels": panels})
		assert.Contains(t, text, "the key must match")
	})
	t.Run("panels as a string", func(t *testing.T) {
		panels := `{"p6": ` + strings.ReplaceAll(statPanel, "%s", "P6") + `}`
		result := mustCall(t, session, "perses_set_dashboard_panels", map[string]any{"project": "myproject", "name": "cpu", "panels": panels, "dry_run": true})
		assert.Equal(t, "would_update", result["action"])
		assert.NotContains(t, fake.stored(t), "p6")
	})
}

func TestSetPanelsMovesAndOrder(t *testing.T) {
	tests := []struct {
		name        string
		input       map[string]any
		wantChanges []string
		wantError   string
	}{
		{
			name: "placements without panels",
			input: map[string]any{
				"placements":       map[string]any{"p1": map[string]any{"x": 4, "y": 8, "width": 8, "height": 9}},
				"expected_version": 0,
			},
			wantChanges: []string{"moved panel p1, placed in layout 0 at x=4 y=8 w=8 h=9"},
		},
		{
			name: "placements with empty panels",
			input: map[string]any{
				"panels":     map[string]any{},
				"placements": map[string]any{"p1": map[string]any{"x": 0, "y": 6}},
			},
			wantChanges: []string{"moved panel p1, placed in layout 0 at x=0 y=6 w=12 h=6"},
		},
		{
			name: "move reserves space before automatic placement",
			input: map[string]any{
				"panels":     map[string]any{"a": json.RawMessage(fmt.Sprintf(statPanel, "A", "mean"))},
				"placements": map[string]any{"p1": map[string]any{"x": 0, "y": 0}},
			},
			wantChanges: []string{
				"added panel a, placed in layout 0 at x=12 y=0 w=12 h=6",
				"moved panel p1, placed in layout 0 at x=0 y=0 w=12 h=6",
			},
		},
		{
			name: "move frees original position for automatic placement",
			input: map[string]any{
				"panels":     map[string]any{"a": json.RawMessage(fmt.Sprintf(statPanel, "A", "mean"))},
				"placements": map[string]any{"p1": map[string]any{"x": 12, "y": 0}},
			},
			wantChanges: []string{
				"added panel a, placed in layout 0 at x=0 y=0 w=12 h=6",
				"moved panel p1, placed in layout 0 at x=12 y=0 w=12 h=6",
			},
		},
		{
			name: "unknown placement does not persist mixed edit",
			input: map[string]any{
				"panels":     map[string]any{"a": json.RawMessage(fmt.Sprintf(statPanel, "A", "mean"))},
				"placements": map[string]any{"unknown": map[string]any{"x": 0, "y": 0}},
			},
			wantError: "placements.unknown: panel not found",
		},
		{
			name: "unknown placement without panels",
			input: map[string]any{
				"placements": map[string]any{"unknown": map[string]any{"x": 0, "y": 0}},
			},
			wantError: "placements.unknown: panel not found",
		},
		{name: "both omitted", input: map[string]any{}, wantError: "give at least one panel, placement or key to remove"},
		{
			name:      "both empty",
			input:     map[string]any{"panels": map[string]any{}, "placements": map[string]any{}},
			wantError: "give at least one panel, placement or key to remove",
		},
		{
			name: "stale version",
			input: map[string]any{
				"placements":       map[string]any{"p1": map[string]any{"x": 0, "y": 6}},
				"expected_version": 7,
			},
			wantError: "at version 0, not 7",
		},
		{
			name: "invalid position is not saved",
			input: map[string]any{
				"placements": map[string]any{"p1": map[string]any{"x": 20, "y": 0, "width": 12}},
			},
			wantError: "spec.layouts[0].spec.items[0].x",
		},
		{
			name: "invalid mixed panel does not persist move",
			input: map[string]any{
				"panels":     map[string]any{"p2": json.RawMessage(fmt.Sprintf(statPanel, "P2", "average"))},
				"placements": map[string]any{"p1": map[string]any{"x": 0, "y": 6}},
			},
			wantError: "spec.panels.p2.spec.plugin.spec.calculation",
		},
		{
			name: "unknown order key",
			input: map[string]any{
				"placements": map[string]any{"p1": map[string]any{"x": 0, "y": 6}},
				"order":      []string{"unknown"},
			},
			wantError: "must appear once and refer to panels or placements",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fake := newFakePerses()
			session := newSession(t, fake)
			mustCall(t, session, "perses_create_dashboard", map[string]any{"project": "myproject", "dashboard": validDashboard})
			before := fake.stored(t)
			test.input["project"], test.input["name"] = "myproject", "cpu"
			if test.wantError != "" {
				text := mustFail(t, session, "perses_set_dashboard_panels", test.input)
				assert.Contains(t, text, test.wantError)
				assert.JSONEq(t, before, fake.stored(t))
				fake.mu.Lock()
				assert.Zero(t, fake.requests["PUT /api/v1/projects/myproject/dashboards/cpu"])
				fake.mu.Unlock()
				if strings.Contains(test.wantError, "at least one") {
					assert.False(t, validationOf(t, text).Valid)
					assert.Contains(t, text, "perses_set_dashboard_layout")
				}
				return
			}
			test.input["dry_run"] = true
			preview := mustCall(t, session, "perses_set_dashboard_panels", test.input)
			assert.Equal(t, "would_update", preview["action"])
			assert.JSONEq(t, before, fake.stored(t))
			fake.mu.Lock()
			assert.Zero(t, fake.requests["PUT /api/v1/projects/myproject/dashboards/cpu"])
			fake.mu.Unlock()
			delete(test.input, "dry_run")
			result := mustCall(t, session, "perses_set_dashboard_panels", test.input)
			assert.Equal(t, preview["changes"], result["changes"])
			var changes []string
			for _, change := range result["changes"].([]any) {
				changes = append(changes, change.(string))
			}
			assert.Equal(t, test.wantChanges, changes)
			stored := object(t, fake.stored(t))["spec"].(map[string]any)
			original := object(t, before)["spec"].(map[string]any)
			assert.Equal(t, original["panels"].(map[string]any)["p1"], stored["panels"].(map[string]any)["p1"])
			items := stored["layouts"].([]any)[0].(map[string]any)["spec"].(map[string]any)["items"].([]any)
			assert.Len(t, items, len(stored["panels"].(map[string]any)))
			for key, raw := range test.input["placements"].(map[string]any) {
				placement := raw.(map[string]any)
				found := false
				for _, rawItem := range items {
					item := rawItem.(map[string]any)
					if item["content"].(map[string]any)["$ref"] != "#/spec/panels/"+key {
						continue
					}
					found = true
					assert.EqualValues(t, placement["x"], item["x"])
					assert.EqualValues(t, placement["y"], item["y"])
					width, height := 12, 6
					if value, ok := placement["width"]; ok {
						width = value.(int)
					}
					if value, ok := placement["height"]; ok {
						height = value.(int)
					}
					assert.EqualValues(t, width, item["width"])
					assert.EqualValues(t, height, item["height"])
				}
				assert.True(t, found, "missing layout item for %s", key)
			}
		})
	}

	for _, stringInput := range []bool{false, true} {
		t.Run(fmt.Sprintf("explicit order string=%v", stringInput), func(t *testing.T) {
			fake := newFakePerses()
			session := newSession(t, fake)
			mustCall(t, session, "perses_create_dashboard", map[string]any{"project": "myproject", "dashboard": validDashboard})
			raw := `{"b":` + fmt.Sprintf(statPanel, "B", "mean") + `,"a":` + fmt.Sprintf(statPanel, "A", "mean") + `,"c":` + fmt.Sprintf(statPanel, "C", "mean") + `}`
			var panels any = json.RawMessage(raw)
			if stringInput {
				panels = raw
			}
			result := mustCall(t, session, "perses_set_dashboard_panels", map[string]any{
				"project": "myproject", "name": "cpu", "panels": panels, "layout": "Ordered", "order": []string{"b", "a", "c"},
			})
			assert.Equal(t, []any{
				"added panel b, placed in layout 1 at x=0 y=0 w=12 h=6",
				"added panel a, placed in layout 1 at x=12 y=0 w=12 h=6",
				"added panel c, placed in layout 1 at x=0 y=6 w=12 h=6",
			}, result["changes"])
		})
	}

	t.Run("placement dry run", func(t *testing.T) {
		fake := newFakePerses()
		session := newSession(t, fake)
		mustCall(t, session, "perses_create_dashboard", map[string]any{"project": "myproject", "dashboard": validDashboard})
		before := fake.stored(t)
		result := mustCall(t, session, "perses_set_dashboard_panels", map[string]any{
			"project": "myproject", "name": "cpu", "dry_run": true,
			"placements": map[string]any{"p1": map[string]any{"x": 0, "y": 6}},
		})
		assert.Equal(t, "would_update", result["action"])
		assert.Equal(t, []any{"moved panel p1, placed in layout 0 at x=0 y=6 w=12 h=6"}, result["changes"])
		assert.JSONEq(t, before, fake.stored(t))
	})
}

func TestSetPanelsPlacementKeepsGroupAndSize(t *testing.T) {
	const twoGroups = `{
	"kind": "Dashboard",
	"metadata": {"name": "cpu"},
	"spec": {
		"duration": "1h",
		"panels": {
			"p1": {"kind": "Panel", "spec": {"display": {"name": "CPU"}, "plugin": {"kind": "StatChart", "spec": {"calculation": "mean"}}}},
			"p2": {"kind": "Panel", "spec": {"display": {"name": "Mem"}, "plugin": {"kind": "StatChart", "spec": {"calculation": "mean"}}}}
		},
		"layouts": [
			{"kind": "Grid", "spec": {"display": {"title": "First"}, "items": [{"x": 0, "y": 0, "width": 24, "height": 8, "content": {"$ref": "#/spec/panels/p1"}}]}},
			{"kind": "Grid", "spec": {"display": {"title": "Second"}, "items": [{"x": 0, "y": 0, "width": 12, "height": 6, "content": {"$ref": "#/spec/panels/p2"}}]}}
		]
	}
}`
	tests := []struct {
		name       string
		input      map[string]any
		wantChange string
		wantLayout int
		wantItem   map[string]any
	}{
		{
			name:       "x and y only keep the group and size",
			input:      map[string]any{"placements": map[string]any{"p1": map[string]any{"x": 0, "y": 4}}},
			wantChange: "moved panel p1, placed in layout 0 at x=0 y=4 w=24 h=8",
			wantLayout: 0,
			wantItem:   map[string]any{"x": 0, "y": 4, "width": 24, "height": 8},
		},
		{
			name:       "explicit width keeps the height",
			input:      map[string]any{"placements": map[string]any{"p1": map[string]any{"x": 0, "y": 0, "width": 6}}},
			wantChange: "moved panel p1, placed in layout 0 at x=0 y=0 w=6 h=8",
			wantLayout: 0,
			wantItem:   map[string]any{"x": 0, "y": 0, "width": 6, "height": 8},
		},
		{
			name:       "replacing the panel keeps the group and size",
			input:      map[string]any{"panels": map[string]any{"p1": json.RawMessage(fmt.Sprintf(statPanel, "CPU bis", "mean"))}, "placements": map[string]any{"p1": map[string]any{"x": 0, "y": 2}}},
			wantChange: "replaced panel p1, placed in layout 0 at x=0 y=2 w=24 h=8",
			wantLayout: 0,
			wantItem:   map[string]any{"x": 0, "y": 2, "width": 24, "height": 8},
		},
		{
			name:       "explicit layout moves to that group",
			input:      map[string]any{"layout": "Second", "placements": map[string]any{"p1": map[string]any{"x": 0, "y": 6}}},
			wantChange: "moved panel p1, placed in layout 1 at x=0 y=6 w=24 h=8",
			wantLayout: 1,
			wantItem:   map[string]any{"x": 0, "y": 6, "width": 24, "height": 8},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fake := newFakePerses()
			session := newSession(t, fake)
			mustCall(t, session, "perses_create_dashboard", map[string]any{"project": "myproject", "dashboard": twoGroups})
			test.input["project"], test.input["name"] = "myproject", "cpu"
			result := mustCall(t, session, "perses_set_dashboard_panels", test.input)
			assert.Equal(t, []any{test.wantChange}, result["changes"])
			first := object(t, fake.stored(t))["spec"]
			result = mustCall(t, session, "perses_set_dashboard_panels", test.input)
			assert.Equal(t, []any{test.wantChange}, result["changes"], "a repeated call must be a no-op")
			assert.Equal(t, first, object(t, fake.stored(t))["spec"])
			layouts := object(t, fake.stored(t))["spec"].(map[string]any)["layouts"].([]any)
			require.Len(t, layouts, 2)
			for i, rawLayout := range layouts {
				for _, rawItem := range rawLayout.(map[string]any)["spec"].(map[string]any)["items"].([]any) {
					item := rawItem.(map[string]any)
					if item["content"].(map[string]any)["$ref"] != "#/spec/panels/p1" {
						continue
					}
					assert.Equal(t, test.wantLayout, i)
					for field, want := range test.wantItem {
						assert.EqualValues(t, want, item[field], field)
					}
				}
			}
		})
	}
}

func TestSetPanelsPlacementKeepsTab(t *testing.T) {
	const withTabs = `{
	"kind": "Dashboard",
	"metadata": {"name": "cpu"},
	"spec": {
		"duration": "1h",
		"panels": {
			"p1": {"kind": "Panel", "spec": {"display": {"name": "CPU"}, "plugin": {"kind": "StatChart", "spec": {"calculation": "mean"}}}},
			"p2": {"kind": "Panel", "spec": {"display": {"name": "Mem"}, "plugin": {"kind": "StatChart", "spec": {"calculation": "mean"}}}},
			"p3": {"kind": "Panel", "spec": {"display": {"name": "Disk"}, "plugin": {"kind": "StatChart", "spec": {"calculation": "mean"}}}}
		},
		"layouts": [
			{"kind": "Tabs", "spec": {"display": {"title": "Tabbed"}, "tabs": [
				{"name": "A", "items": [{"x": 0, "y": 0, "width": 24, "height": 8, "content": {"$ref": "#/spec/panels/p1"}}]},
				{"name": "B", "items": [{"x": 0, "y": 0, "width": 8, "height": 5, "content": {"$ref": "#/spec/panels/p2"}}]}
			]}},
			{"kind": "Grid", "spec": {"display": {"title": "Last"}, "items": [{"x": 0, "y": 0, "width": 12, "height": 6, "content": {"$ref": "#/spec/panels/p3"}}]}}
		]
	}
}`
	// location is "layout/tab" for a tab item, "layout" for a grid item.
	tests := []struct {
		name         string
		input        map[string]any
		wantChange   string
		wantLocation string
		wantItem     map[string]any
	}{
		{
			name:         "x and y only keep the tab and size",
			input:        map[string]any{"placements": map[string]any{"p2": map[string]any{"x": 4, "y": 3}}},
			wantChange:   "moved panel p2, placed in layout 0 tab 1 at x=4 y=3 w=8 h=5",
			wantLocation: "0/1",
			wantItem:     map[string]any{"x": 4, "y": 3, "width": 8, "height": 5},
		},
		{
			name:         "replacing the panel with a placement keeps the tab",
			input:        map[string]any{"panels": map[string]any{"p2": json.RawMessage(fmt.Sprintf(statPanel, "Mem bis", "mean"))}, "placements": map[string]any{"p2": map[string]any{"x": 0, "y": 2, "height": 7}}},
			wantChange:   "replaced panel p2, placed in layout 0 tab 1 at x=0 y=2 w=8 h=7",
			wantLocation: "0/1",
			wantItem:     map[string]any{"x": 0, "y": 2, "width": 8, "height": 7},
		},
		{
			name:         "replacing the panel without placement keeps its position",
			input:        map[string]any{"panels": map[string]any{"p2": json.RawMessage(fmt.Sprintf(statPanel, "Mem bis", "mean"))}},
			wantChange:   "replaced panel p2, kept its position",
			wantLocation: "0/1",
			wantItem:     map[string]any{"x": 0, "y": 0, "width": 8, "height": 5},
		},
		{
			name:         "explicit layout moves out of the tab",
			input:        map[string]any{"layout": "Last", "placements": map[string]any{"p2": map[string]any{"x": 12, "y": 0}}},
			wantChange:   "moved panel p2, placed in layout 1 at x=12 y=0 w=8 h=5",
			wantLocation: "1",
			wantItem:     map[string]any{"x": 12, "y": 0, "width": 8, "height": 5},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fake := newFakePerses()
			session := newSession(t, fake)
			mustCall(t, session, "perses_create_dashboard", map[string]any{"project": "myproject", "dashboard": withTabs})
			test.input["project"], test.input["name"] = "myproject", "cpu"
			result := mustCall(t, session, "perses_set_dashboard_panels", test.input)
			assert.Equal(t, []any{test.wantChange}, result["changes"])

			layouts := object(t, fake.stored(t))["spec"].(map[string]any)["layouts"].([]any)
			require.Len(t, layouts, 2)
			found := map[string]map[string]any{}
			collect := func(location string, items []any) {
				for _, rawItem := range items {
					item := rawItem.(map[string]any)
					found[item["content"].(map[string]any)["$ref"].(string)+"@"+location] = item
				}
			}
			for i, tab := range layouts[0].(map[string]any)["spec"].(map[string]any)["tabs"].([]any) {
				collect(fmt.Sprintf("0/%d", i), tab.(map[string]any)["items"].([]any))
			}
			collect("1", layouts[1].(map[string]any)["spec"].(map[string]any)["items"].([]any))

			require.Len(t, found, 3, "each panel must be placed exactly once: %v", found)
			assert.Contains(t, found, "#/spec/panels/p1@0/0")
			assert.Contains(t, found, "#/spec/panels/p3@1")
			item, ok := found["#/spec/panels/p2@"+test.wantLocation]
			require.True(t, ok, "p2 not at %s: %v", test.wantLocation, found)
			for field, want := range test.wantItem {
				assert.EqualValues(t, want, item[field], field)
			}
		})
	}
}

func TestSetPanelsPanelPlacedTwice(t *testing.T) {
	const placedTwice = `{
	"kind": "Dashboard",
	"metadata": {"name": "cpu"},
	"spec": {
		"duration": "1h",
		"panels": {
			"p1": {"kind": "Panel", "spec": {"display": {"name": "CPU"}, "plugin": {"kind": "StatChart", "spec": {"calculation": "mean"}}}}
		},
		"layouts": [
			{"kind": "Grid", "spec": {"display": {"title": "Overview"}, "items": [{"x": 0, "y": 0, "width": 12, "height": 6, "content": {"$ref": "#/spec/panels/p1"}}]}},
			{"kind": "Tabs", "spec": {"display": {"title": "Tabbed"}, "tabs": [
				{"name": "A", "items": [{"x": 4, "y": 2, "width": 8, "height": 5, "content": {"$ref": "#/spec/panels/p1"}}]}
			]}}
		]
	}
}`
	fake := newFakePerses()
	session := newSession(t, fake)
	mustCall(t, session, "perses_create_dashboard", map[string]any{"project": "myproject", "dashboard": placedTwice})
	before := fake.stored(t)

	for name, input := range map[string]map[string]any{
		"move":                   {"placements": map[string]any{"p1": map[string]any{"x": 12, "y": 0}}},
		"replace with placement": {"panels": map[string]any{"p1": json.RawMessage(fmt.Sprintf(statPanel, "CPU bis", "mean"))}, "placements": map[string]any{"p1": map[string]any{"x": 12, "y": 0}}},
	} {
		t.Run(name, func(t *testing.T) {
			input["project"], input["name"] = "myproject", "cpu"
			text := mustFail(t, session, "perses_set_dashboard_panels", input)
			assert.Contains(t, text, `placements.p1: the panel appears in 2 places (layout 0 "Overview", layout 1 "Tabbed" tab 0 "A")`)
			assert.Contains(t, text, "perses_set_dashboard_layout")
			assert.JSONEq(t, before, fake.stored(t), "the dashboard must not change")
		})
	}

	t.Run("replace without placement keeps both items", func(t *testing.T) {
		result := mustCall(t, session, "perses_set_dashboard_panels", map[string]any{
			"project": "myproject", "name": "cpu",
			"panels": map[string]any{"p1": json.RawMessage(fmt.Sprintf(statPanel, "CPU bis", "mean"))},
		})
		assert.Equal(t, []any{"replaced panel p1, kept its position"}, result["changes"])
		stored, wanted := object(t, fake.stored(t)), object(t, before)
		assert.Equal(t, wanted["spec"].(map[string]any)["layouts"], stored["spec"].(map[string]any)["layouts"])
		assert.Equal(t, "CPU bis", stored["spec"].(map[string]any)["panels"].(map[string]any)["p1"].(map[string]any)["spec"].(map[string]any)["display"].(map[string]any)["name"])
	})
}

func TestEditToolHints(t *testing.T) {
	d := &dashboard{}
	for _, test := range []struct {
		tool       *tools.Tool
		idempotent bool
	}{
		{d.SetPanels(), true},
		{d.SetVariables(), true},
		{d.SetLayout(), false},
	} {
		annotations := test.tool.MCPTool.Annotations
		assert.Equal(t, test.idempotent, annotations.IdempotentHint, test.tool.MCPTool.Name)
		assert.True(t, *annotations.DestructiveHint, test.tool.MCPTool.Name)
		assert.False(t, annotations.ReadOnlyHint, test.tool.MCPTool.Name)
	}
}

func TestRemovePanels(t *testing.T) {
	fake := newFakePerses()
	session := newSession(t, fake)
	mustCall(t, session, "perses_create_dashboard", map[string]any{"project": "myproject", "dashboard": validDashboard})

	result := mustCall(t, session, "perses_set_dashboard_panels", map[string]any{"project": "myproject", "name": "cpu", "remove": []string{"p1", "nope"}})
	assert.Equal(t, []any{"removed panel p1 and 1 layout item(s)", "panel nope not found"}, result["changes"])
	assert.Equal(t, float64(0), result["panels"])
	assert.NotContains(t, fake.stored(t), "#/spec/panels/p1")
}

func TestSetPanelsRemoveAndAddInOneCall(t *testing.T) {
	fake := newFakePerses()
	session := newSession(t, fake)
	mustCall(t, session, "perses_create_dashboard", map[string]any{"project": "myproject", "dashboard": validDashboard})

	panels := map[string]any{"p2": object(t, strings.ReplaceAll(statPanel, "%s", "P2"))}
	result := mustCall(t, session, "perses_set_dashboard_panels", map[string]any{"project": "myproject", "name": "cpu", "panels": panels, "remove": []string{"p1"}})
	// The removal frees x=0 y=0 before the new panel is placed.
	assert.Equal(t, []any{"removed panel p1 and 1 layout item(s)", "added panel p2, placed in layout 0 at x=0 y=0 w=12 h=6"}, result["changes"])
	assert.Equal(t, float64(1), result["panels"])

	text := mustFail(t, session, "perses_set_dashboard_panels", map[string]any{"project": "myproject", "name": "cpu", "panels": panels, "remove": []string{"p2"}})
	assert.Contains(t, text, "either removed or set")
	text = mustFail(t, session, "perses_set_dashboard_panels", map[string]any{"project": "myproject", "name": "cpu"})
	assert.Contains(t, text, "give at least one panel, placement or key to remove")
}

func TestSetPanelsRemoveIsAtomic(t *testing.T) {
	fake := newFakePerses()
	session := newSession(t, fake)
	mustCall(t, session, "perses_create_dashboard", map[string]any{"project": "myproject", "dashboard": validDashboard})
	before := fake.stored(t)

	invalidPanel := strings.Replace(strings.ReplaceAll(statPanel, "%s", "P2"), "P2\"}}", "average\"}}", 1)
	for _, test := range []struct {
		name      string
		args      map[string]any
		wantError string
	}{
		{
			name:      "removed key also placed",
			args:      map[string]any{"remove": []string{"p1"}, "placements": map[string]any{"p1": map[string]any{"x": 0, "y": 0}}},
			wantError: "either removed or set",
		},
		{
			name:      "order without panels or placements",
			args:      map[string]any{"remove": []string{"p1"}, "order": []string{"p1"}},
			wantError: "order: only applies",
		},
		{
			name:      "removal with a panel rejected by the schemas",
			args:      map[string]any{"remove": []string{"p1"}, "panels": map[string]any{"p2": object(t, invalidPanel)}},
			wantError: "spec.panels.p2.spec.plugin.spec.calculation",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			args := map[string]any{"project": "myproject", "name": "cpu"}
			for key, value := range test.args {
				args[key] = value
			}
			text := mustFail(t, session, "perses_set_dashboard_panels", args)
			assert.Contains(t, text, test.wantError)
			assert.Equal(t, before, fake.stored(t), "a failed edit must not remove anything")
		})
	}

	t.Run("dry run removal", func(t *testing.T) {
		result := mustCall(t, session, "perses_set_dashboard_panels", map[string]any{"project": "myproject", "name": "cpu", "remove": []string{"p1"}, "dry_run": true})
		assert.Equal(t, "would_update", result["action"])
		assert.Equal(t, float64(0), result["panels"])
		assert.Equal(t, before, fake.stored(t))
	})
}

func TestVariables(t *testing.T) {
	fake := newFakePerses()
	session := newSession(t, fake)
	mustCall(t, session, "perses_create_dashboard", map[string]any{"project": "myproject", "dashboard": validDashboard})

	variables := `[
		{"kind": "TextVariable", "spec": {"name": "env", "value": "prod"}},
		{"kind": "ListVariable", "spec": {"name": "inst", "plugin": {"kind": "PrometheusLabelValuesVariable", "spec": {"labelName": "instance"}}}}
	]`
	result := mustCall(t, session, "perses_set_dashboard_variables", map[string]any{"project": "myproject", "name": "cpu", "variables": variables})
	assert.Equal(t, []any{"added variable env", "added variable inst"}, result["changes"])
	assert.Equal(t, float64(2), result["variables"])

	result = mustCall(t, session, "perses_set_dashboard_variables", map[string]any{"project": "myproject", "name": "cpu",
		"variables": []any{object(t, `{"kind": "TextVariable", "spec": {"name": "env", "value": "staging"}}`)}})
	assert.Equal(t, []any{"replaced variable env"}, result["changes"])
	assert.Equal(t, float64(2), result["variables"])
	assert.Contains(t, fake.stored(t), "staging")

	result = mustCall(t, session, "perses_set_dashboard_variables", map[string]any{"project": "myproject", "name": "cpu", "remove": []string{"env", "nope"}})
	assert.Equal(t, []any{"removed variable env", "variable nope not found"}, result["changes"])
	assert.Equal(t, float64(1), result["variables"])

	result = mustCall(t, session, "perses_set_dashboard_variables", map[string]any{"project": "myproject", "name": "cpu", "remove": []string{"inst"},
		"variables": []any{object(t, `{"kind": "TextVariable", "spec": {"name": "environment", "value": "production"}}`)}})
	assert.Equal(t, []any{"removed variable inst", "added variable environment"}, result["changes"])
	assert.Equal(t, float64(1), result["variables"])

	stored := fake.stored(t)
	text := mustFail(t, session, "perses_set_dashboard_variables", map[string]any{"project": "myproject", "name": "cpu", "remove": []string{"environment"},
		"variables": []any{object(t, `{"kind": "TextVariable", "spec": {"name": "environment", "value": "x"}}`)}})
	assert.Contains(t, text, "either removed or set")
	// A removal combined with a variable rejected by the schemas must not remove anything.
	text = mustFail(t, session, "perses_set_dashboard_variables", map[string]any{"project": "myproject", "name": "cpu", "remove": []string{"environment"},
		"variables": `[{"kind": "TextVariable", "spec": {"value": "x"}}]`})
	assert.Contains(t, text, "variables")
	text = mustFail(t, session, "perses_set_dashboard_variables", map[string]any{"project": "myproject", "name": "cpu"})
	assert.Contains(t, text, "at least one variable")
	text = mustFail(t, session, "perses_set_dashboard_variables", map[string]any{"project": "myproject", "name": "cpu", "variables": []any{}})
	assert.Contains(t, text, "at least one variable")
	assert.Equal(t, stored, fake.stored(t), "a rejected variable edit must not change the dashboard")

	result = mustCall(t, session, "perses_set_dashboard_variables", map[string]any{"project": "myproject", "name": "cpu", "variables": []any{}, "remove": []string{"environment"}, "dry_run": true})
	assert.Equal(t, "would_update", result["action"])
	assert.Equal(t, float64(0), result["variables"])
	assert.Equal(t, stored, fake.stored(t))

	text = mustFail(t, session, "perses_set_dashboard_variables", map[string]any{"project": "myproject", "name": "cpu", "variables": `[{"kind": "TextVariable", "spec": {"value": "x"}}]`})
	assert.Contains(t, text, "variables")
}

func TestSetLayout(t *testing.T) {
	fake := newFakePerses()
	session := newSession(t, fake)
	mustCall(t, session, "perses_create_dashboard", map[string]any{"project": "myproject", "dashboard": validDashboard})

	layout := `{"kind": "Grid", "spec": {"display": {"title": "Second"}, "items": []}}`
	result := mustCall(t, session, "perses_set_dashboard_layout", map[string]any{"project": "myproject", "name": "cpu", "layout": layout})
	assert.Equal(t, []any{`added layout 1 "Second"`}, result["changes"])

	result = mustCall(t, session, "perses_set_dashboard_layout", map[string]any{"project": "myproject", "name": "cpu", "layout": strings.Replace(layout, "Second", "second", 1)})
	assert.Equal(t, []any{`replaced layout 1 "second"`}, result["changes"])
	assert.Equal(t, float64(2), result["layouts"])

	result = mustCall(t, session, "perses_set_dashboard_layout", map[string]any{"project": "myproject", "name": "cpu", "index": 0, "layout": layout})
	assert.Equal(t, []any{`replaced layout 0 "Second"`}, result["changes"])
	// p1 is no longer displayed.
	require.Len(t, result["warnings"], 1)

	result = mustCall(t, session, "perses_set_dashboard_layout", map[string]any{"project": "myproject", "name": "cpu", "index": 1, "remove": true})
	assert.Equal(t, []any{`removed layout 1 "second"`}, result["changes"])
	assert.Equal(t, float64(1), result["layouts"])

	text := mustFail(t, session, "perses_set_dashboard_layout", map[string]any{"project": "myproject", "name": "cpu", "index": 5, "layout": layout})
	assert.Contains(t, text, "out of range")
	text = mustFail(t, session, "perses_set_dashboard_layout", map[string]any{"project": "myproject", "name": "cpu"})
	assert.Contains(t, text, "layout is required")
}

func TestGetAndList(t *testing.T) {
	fake := newFakePerses()
	session := newSession(t, fake)
	mustCall(t, session, "perses_create_dashboard", map[string]any{"project": "myproject", "dashboard": validDashboard})

	full := mustCall(t, session, "perses_get_dashboard_by_name", map[string]any{"project": "myproject", "name": "cpu", "summary": false})
	assert.Equal(t, "Dashboard", full["kind"])
	assert.Contains(t, full, "spec")

	outline := mustCall(t, session, "perses_get_dashboard_by_name", map[string]any{"project": "myproject", "name": "cpu"})
	assert.Nil(t, outline["kind"])
	assert.NotContains(t, outline, "spec")
	panels := outline["panelList"].([]any)
	require.Len(t, panels, 1)
	assert.Equal(t, "p1", panels[0].(map[string]any)["key"])
	assert.Equal(t, []any{"layout 0: x=0 y=0 w=12 h=6"}, panels[0].(map[string]any)["placement"])
	explicitOutline := mustCall(t, session, "perses_get_dashboard_by_name", map[string]any{"project": "myproject", "name": "cpu", "summary": true})
	assert.Equal(t, outline, explicitOutline)

	for _, summary := range []any{nil, true, false} {
		arguments := map[string]any{"project": "myproject", "name": "cpu", "panels": []string{"p1", "nope"}}
		if summary != nil {
			arguments["summary"] = summary
		}
		some := mustCall(t, session, "perses_get_dashboard_by_name", arguments)
		assert.Len(t, some["panels"], 1)
		assert.Equal(t, []any{"nope"}, some["missing"])
		assert.Equal(t, map[string]any{"p1": "CPU"}, some["available"], "missing keys come with the available ones")
		panel := some["panels"].(map[string]any)["p1"].(map[string]any)
		plugin := panel["spec"].(map[string]any)["plugin"].(map[string]any)
		assert.Equal(t, "mean", plugin["spec"].(map[string]any)["calculation"])
	}

	found := mustCall(t, session, "perses_get_dashboard_by_name", map[string]any{"project": "myproject", "name": "cpu", "panels": []string{"p1"}})
	assert.NotContains(t, found, "available", "available is only returned when a key is missing")

	for _, empty := range [][]string{{}, {" "}} {
		outlined := mustCall(t, session, "perses_get_dashboard_by_name", map[string]any{"project": "myproject", "name": "cpu", "summary": false, "panels": empty})
		assert.Nil(t, outlined["kind"], "an empty panel selection must not fall back to the full dashboard")
		assert.NotEmpty(t, outlined["panelList"], "an empty panel selection returns the outline")
	}

	for _, guessed := range [][]string{{""}, {"nope"}} {
		fallback := mustCall(t, session, "perses_get_dashboard_by_name", map[string]any{"project": "myproject", "name": "cpu", "panels": guessed})
		assert.NotEmpty(t, fallback["panelList"], "guessed keys must fall back to the outline")
	}

	text := mustFail(t, session, "perses_get_dashboard_by_name", map[string]any{"project": "myproject", "name": "nope"})
	assert.Contains(t, text, "does not exist")

	list, isError := callTool(t, session, "perses_list_dashboards", map[string]any{"project": "myproject"})
	require.False(t, isError)
	var listed struct {
		Count      int              `json:"count"`
		Dashboards []map[string]any `json:"dashboards"`
	}
	require.NoError(t, json.Unmarshal([]byte(list), &listed))
	assert.Equal(t, 1, listed.Count)
	require.Len(t, listed.Dashboards, 1)
	assert.Equal(t, "cpu", listed.Dashboards[0]["name"])
	assert.Equal(t, float64(1), listed.Dashboards[0]["panels"])
}

func TestDashboardLinksOmitCredentials(t *testing.T) {
	srv := httptest.NewServer(newFakePerses())
	t.Cleanup(srv.Close)
	base := common.MustParseURL(strings.Replace(srv.URL, "://", "://user:secret@", 1))
	client := apiClient.NewWithClient(&perseshttp.RESTClient{BaseURL: base, Client: srv.Client()})
	session := connect(t, client)

	created := mustCall(t, session, "perses_create_dashboard", map[string]any{"project": "myproject", "dashboard": validDashboard})
	outline := mustCall(t, session, "perses_get_dashboard_by_name", map[string]any{"project": "myproject", "name": "cpu"})
	for _, link := range []any{created["url"], outline["url"]} {
		assert.Equal(t, srv.URL+"/projects/myproject/dashboards/cpu", link)
	}
	assert.Equal(t, "user:secret", client.RESTClient().BaseURL.User.String())
}

func TestToolDescriptionsStayCompact(t *testing.T) {
	// Keep tool descriptions compact.
	for _, tool := range New(nil).GetTools() {
		schema, ok := tool.MCPTool.InputSchema.(*jsonschema.Schema)
		require.True(t, ok, "expected a JSON Schema for %s", tool.MCPTool.Name)
		if versionSchema := schema.Properties[expectedVersionProperty]; versionSchema != nil {
			assert.Contains(t, versionSchema.Description, "best-effort check, not a lock", tool.MCPTool.Name)
		}
		if tool.IsWriteTool && tool.MCPTool.Name != "perses_delete_dashboard" {
			assert.NotContains(t, tool.MCPTool.Description, "Model:", tool.MCPTool.Name)
			assert.Less(t, len(tool.MCPTool.Description), 1100, tool.MCPTool.Name)
		}
	}
}

func TestGetMissingPanelsOnEmptyDashboard(t *testing.T) {
	fake := newFakePerses()
	session := newSession(t, fake)
	empty := strings.Replace(validDashboard, `"panels": {"p1"`, `"unused": {"p1"`, 1)
	var dash map[string]any
	require.NoError(t, json.Unmarshal([]byte(empty), &dash))
	spec := dash["spec"].(map[string]any)
	delete(spec, "unused")
	spec["layouts"] = []any{}
	mustCall(t, session, "perses_create_dashboard", map[string]any{"project": "myproject", "dashboard": dash})

	result := mustCall(t, session, "perses_get_dashboard_by_name", map[string]any{"project": "myproject", "name": "cpu", "panels": []string{"p1"}})
	// None of the requested keys exists: the outline (with its empty panelList) comes back with missing.
	assert.Equal(t, []any{"p1"}, result["missing"])
	assert.Nil(t, result["panelList"])
	assert.Nil(t, result["kind"])
}

func TestAvailablePanelsWithoutDisplay(t *testing.T) {
	dash := &v1.Dashboard{}
	dash.Spec.Panels = map[string]*dashboardSpec.Panel{"nil": nil, "unnamed": {Kind: "Panel"}}
	assert.Equal(t, map[string]string{"nil": "", "unnamed": ""}, availablePanels(dash))
}
