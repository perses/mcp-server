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
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
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

func TestGetTools(t *testing.T) {
	names := []string{}
	for _, tool := range New(nil).GetTools() {
		names = append(names, tool.MCPTool.Name)
		assert.NotNil(t, tool.MCPTool.InputSchema, tool.MCPTool.Name)
	}
	assert.Equal(t, []string{
		"perses_list_dashboards", "perses_get_dashboard_by_name", "perses_create_dashboard", "perses_update_dashboard",
		"perses_delete_dashboard",
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
	t.Run("string input is rejected", func(t *testing.T) {
		fake := newFakePerses()
		session := newSession(t, fake)
		result, err := session.CallTool(context.Background(), &mcp.CallToolParams{
			Name: "perses_create_dashboard", Arguments: map[string]any{"project": "myproject", "dashboard": validDashboard},
		})
		require.True(t, err != nil || result.IsError, "a dashboard given as a string must be rejected")
		assert.Empty(t, fake.dashboards)
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
		assert.Equal(t, "myproject", result["project"], "the dashboard omits metadata.project: the input project is used")
		assert.True(t, strings.HasSuffix(result["url"].(string), "/projects/myproject/dashboards/cpu"), result["url"])
		assert.Empty(t, fake.dashboards)
	})
	t.Run("missing kind is rejected", func(t *testing.T) {
		fake := newFakePerses()
		session := newSession(t, fake)
		withoutKind := map[string]any{}
		for key, value := range asObject {
			if key != "kind" {
				withoutKind[key] = value
			}
		}
		text := mustFail(t, session, "perses_create_dashboard", map[string]any{"project": "myproject", "dashboard": withoutKind})
		assert.Contains(t, text, `invalid kind: "" for a Dashboard type`)
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
	t.Run("validation error is the error of the Perses client", func(t *testing.T) {
		fake := newFakePerses()
		session := newSession(t, fake)
		invalid := strings.ReplaceAll(validDashboard, `p1`, `p2`)
		invalid = strings.Replace(invalid, `"mean"`, `"average"`, 1)
		text := mustFail(t, session, "perses_create_dashboard", map[string]any{"project": "myproject", "dashboard": object(t, invalid)})
		assert.Contains(t, text, "invalid panel p2: spec.calculation: conflicting values \"mean\" and \"average\"")
		assert.Contains(t, text, "StatusCode: 400")
		assert.Empty(t, fake.dashboards)
	})
	t.Run("decode error is the message of the Perses model", func(t *testing.T) {
		fake := newFakePerses()
		session := newSession(t, fake)
		wrongKind := strings.Replace(validDashboard, `"kind": "Dashboard"`, `"kind": "Datasource"`, 1)
		text := mustFail(t, session, "perses_create_dashboard", map[string]any{"project": "myproject", "dashboard": object(t, wrongKind)})
		assert.Equal(t, `invalid kind: "Datasource" for a Dashboard type`, text)
		assert.Zero(t, fake.requests["POST /api/validate/dashboards"])
	})
}

func TestUpdate(t *testing.T) {
	fake := newFakePerses()
	session := newSession(t, fake)
	mustCall(t, session, "perses_create_dashboard", map[string]any{"project": "myproject", "dashboard": object(t, validDashboard)})

	t.Run("missing dashboard", func(t *testing.T) {
		missing := strings.Replace(validDashboard, `"cpu"`, `"other"`, 1)
		for _, extra := range []map[string]any{{}, {"expected_version": 0}, {"dry_run": true}} {
			args := map[string]any{"project": "myproject", "dashboard": object(t, missing)}
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
		result := mustCall(t, session, "perses_update_dashboard", map[string]any{"project": "myproject", "dashboard": object(t, validDashboard)})
		assert.Equal(t, "updated", result["action"])
		assert.Equal(t, before, fake.requests["GET /api/v1/projects/myproject/dashboards/cpu"], "an update-only role must be able to update")
	})
	t.Run("version mismatch", func(t *testing.T) {
		text := mustFail(t, session, "perses_update_dashboard", map[string]any{"project": "myproject", "dashboard": object(t, validDashboard), "expected_version": 7})
		assert.Contains(t, text, "is at version 1, not 7")
	})
	t.Run("update", func(t *testing.T) {
		result := mustCall(t, session, "perses_update_dashboard", map[string]any{"project": "myproject", "dashboard": object(t, validDashboard), "expected_version": 1})
		assert.Equal(t, "updated", result["action"])
		assert.Equal(t, float64(2), result["version"])
	})
	t.Run("dry run saves nothing", func(t *testing.T) {
		fake := newFakePerses()
		session := newSession(t, fake)
		mustCall(t, session, "perses_create_dashboard", map[string]any{"project": "myproject", "dashboard": object(t, validDashboard)})
		before, err := json.Marshal(fake.dashboards["cpu"])
		require.NoError(t, err)

		renamed := strings.Replace(validDashboard, `"name": "CPU"`, `"name": "Renamed"`, 1)
		result := mustCall(t, session, "perses_update_dashboard", map[string]any{"project": "myproject", "dashboard": object(t, renamed), "dry_run": true, "expected_version": 0})
		assert.Equal(t, "would_update", result["action"])
		assert.Equal(t, true, result["dryRun"])
		assert.Zero(t, fake.requests["PUT /api/v1/projects/myproject/dashboards/cpu"])
		after, err := json.Marshal(fake.dashboards["cpu"])
		require.NoError(t, err)
		assert.JSONEq(t, string(before), string(after))
		assert.Equal(t, float64(0), fake.dashboards["cpu"]["metadata"].(map[string]any)["version"])
	})
}

func TestGetAndList(t *testing.T) {
	fake := newFakePerses()
	session := newSession(t, fake)
	mustCall(t, session, "perses_create_dashboard", map[string]any{"project": "myproject", "dashboard": object(t, validDashboard)})

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

	text := mustFail(t, session, "perses_get_dashboard_by_name", map[string]any{"project": "myproject", "name": "cpu", "panels": []string{""}})
	assert.Contains(t, text, "panels")

	text = mustFail(t, session, "perses_get_dashboard_by_name", map[string]any{"project": "myproject", "name": "cpu", "summary": false, "panels": []string{}})
	assert.Contains(t, text, "panels")
	assert.NotContains(t, text, `"kind"`, "an empty panel selection must not fall back to the full dashboard")

	text = mustFail(t, session, "perses_get_dashboard_by_name", map[string]any{"project": "myproject", "name": "nope"})
	assert.Contains(t, text, "does not exist")

	list, isError := callTool(t, session, "perses_list_dashboards", map[string]any{"project": "myproject"})
	require.False(t, isError)
	var listed []map[string]any
	require.NoError(t, json.Unmarshal([]byte(list), &listed))
	require.Len(t, listed, 1)
	assert.Equal(t, "cpu", listed[0]["name"])
	assert.Equal(t, float64(1), listed[0]["panels"])
}

func TestDashboardLinksOmitCredentials(t *testing.T) {
	srv := httptest.NewServer(newFakePerses())
	t.Cleanup(srv.Close)
	base := common.MustParseURL(strings.Replace(srv.URL, "://", "://user:secret@", 1))
	client := apiClient.NewWithClient(&perseshttp.RESTClient{BaseURL: base, Client: srv.Client()})
	session := connect(t, client)

	created := mustCall(t, session, "perses_create_dashboard", map[string]any{"project": "myproject", "dashboard": object(t, validDashboard)})
	outline := mustCall(t, session, "perses_get_dashboard_by_name", map[string]any{"project": "myproject", "name": "cpu"})
	for _, link := range []any{created["url"], outline["url"]} {
		assert.Equal(t, srv.URL+"/projects/myproject/dashboards/cpu", link)
	}
	assert.Equal(t, "user:secret", client.RESTClient().BaseURL.User.String())
}

func TestToolDescriptionsStayCompact(t *testing.T) {
	// Shared model/workflow guidance belongs in server instructions, not repeated tool descriptions.
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
	assert.Equal(t, []any{"p1"}, result["missing"])
	assert.Equal(t, map[string]any{}, result["available"], "an empty dashboard reports that no panel is available")
}

func TestAvailablePanelsWithoutDisplay(t *testing.T) {
	dash := &v1.Dashboard{}
	dash.Spec.Panels = map[string]*dashboardSpec.Panel{"nil": nil, "unnamed": {Kind: "Panel"}}
	assert.Equal(t, map[string]string{"nil": "", "unnamed": ""}, availablePanels(dash))
}
