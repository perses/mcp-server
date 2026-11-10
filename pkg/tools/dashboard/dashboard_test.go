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
	"github.com/perses/mcp-server/pkg/tools/internal/tooltest"
	apiClient "github.com/perses/perses/pkg/client/api"
	"github.com/perses/perses/pkg/client/perseshttp"
	v1 "github.com/perses/perses/pkg/model/api/v1"
	"github.com/perses/spec/go/common"
	dashboardSpec "github.com/perses/spec/go/dashboard"
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

// withoutMetadata returns the valid dashboard without its metadata key.
func withoutMetadata(t *testing.T) map[string]any {
	t.Helper()
	dash := object(t, validDashboard)
	delete(dash, "metadata")
	return dash
}

// fakePerses is an in-memory Perses API storing dashboards as raw JSON documents. Like the real server, it validates
// dashboards on the validation endpoint and when creating or updating them, rejecting the calculation "average".
// The embedded tooltest.Perses records the requests.
type fakePerses struct {
	tooltest.Perses
	mu         sync.Mutex
	dashboards map[string]map[string]any
	getStatus  int
	// validated is the last dashboard sent to the validation endpoint.
	validated map[string]any
}

func newFakePerses() *fakePerses {
	fake := &fakePerses{dashboards: map[string]map[string]any{}}
	fake.Handler = http.HandlerFunc(fake.serve)
	return fake
}

func (f *fakePerses) serve(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	body, _ := io.ReadAll(r.Body)

	if r.Method == http.MethodPost && r.URL.Path == "/api/validate/dashboards" {
		f.validated = decode(body)
		if invalid(body) {
			// The validation endpoint wraps the error of the validation once more.
			f.writeMessage(w, http.StatusBadRequest, "bad request: "+invalidPanelMessage)
			return
		}
		w.WriteHeader(http.StatusOK)
		return
	}
	if (r.Method == http.MethodPost || r.Method == http.MethodPut) && invalid(body) {
		f.writeMessage(w, http.StatusBadRequest, invalidPanelMessage)
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

// invalidPanelMessage is how Perses reports a value rejected by a plugin schema.
const invalidPanelMessage = "bad request: invalid panel p2: spec.calculation: conflicting values \"mean\" and \"average\":\n    1:1\n"

func invalid(body []byte) bool {
	return strings.Contains(string(body), `"average"`)
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

func newSession(t *testing.T, fake *fakePerses) *mcp.ClientSession {
	t.Helper()
	return tooltest.Connect(t, New(tooltest.NewClient(t, fake)).GetTools())
}

// callTool calls a tool and returns its first text content and whether the result is an error.
func callTool(t *testing.T, session *mcp.ClientSession, name string, args map[string]any) (string, bool) {
	t.Helper()
	result, _ := tooltest.CallTool(t, session, name, args)
	if !hasLen(t, result.Content, 1) {
		t.FailNow()
	}
	if result.StructuredContent != nil {
		t.Fatalf("the dashboard tools must not duplicate the payload as structured content: expected nil, got %v", result.StructuredContent)
	}
	text, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("expected text content, got %T", result.Content[0])
	}
	return text.Text, result.IsError
}

func object(t *testing.T, text string) map[string]any {
	t.Helper()
	var value map[string]any
	if err := json.Unmarshal([]byte(text), &value); err != nil {
		t.Fatalf("not a JSON object: %s: %v", text, err)
	}
	return value
}

func mustCall(t *testing.T, session *mcp.ClientSession, name string, args map[string]any) map[string]any {
	t.Helper()
	text, isError := callTool(t, session, name, args)
	if isError {
		t.Fatalf("unexpected tool error: %s", text)
	}
	return object(t, text)
}

func mustFail(t *testing.T, session *mcp.ClientSession, name string, args map[string]any) string {
	t.Helper()
	text, isError := callTool(t, session, name, args)
	if !isError {
		t.Fatalf("expected a tool error, got: %s", text)
	}
	return text
}

func TestGetTools(t *testing.T) {
	names := []string{}
	for _, tool := range New(nil).GetTools() {
		names = append(names, tool.MCPTool.Name)
		if tool.MCPTool.InputSchema == nil {
			t.Errorf("%v: unexpected nil", tool.MCPTool.Name)
		}
	}
	equal(t, []string{
		"perses_list_dashboards", "perses_get_dashboard_by_name", "perses_create_dashboard", "perses_update_dashboard",
		"perses_delete_dashboard",
	}, names)
}

func TestCreate(t *testing.T) {
	var asObject map[string]any
	if err := json.Unmarshal([]byte(validDashboard), &asObject); err != nil {
		t.Fatal(err)
	}

	t.Run("object input returns a summary", func(t *testing.T) {
		fake := newFakePerses()
		session := newSession(t, fake)
		result := mustCall(t, session, "perses_create_dashboard", map[string]any{"project": "myproject", "dashboard": asObject})
		equal(t, "created", result["action"])
		equal(t, "cpu", result["name"])
		equal(t, "myproject", result["project"])
		equal(t, float64(1), result["panels"])
		equal(t, "/projects/myproject/dashboards/cpu", result["url"].(string)[strings.Index(result["url"].(string), "/projects"):])
		if result["dashboard"] != nil {
			t.Errorf("expected nil, got %v", result["dashboard"])
		}
		equal(t, 0, fake.Count(http.MethodPost, "/api/validate/dashboards"), "a real write relies on the validation of Perses")
	})
	t.Run("string input is rejected", func(t *testing.T) {
		fake := newFakePerses()
		session := newSession(t, fake)
		result, err := session.CallTool(context.Background(), &mcp.CallToolParams{
			Name: "perses_create_dashboard", Arguments: map[string]any{"project": "myproject", "dashboard": validDashboard},
		})
		if err == nil && !result.IsError {
			t.Fatalf("a dashboard given as a string must be rejected")
		}
		empty(t, fake.dashboards)
	})
	t.Run("include_dashboard", func(t *testing.T) {
		session := newSession(t, newFakePerses())
		result := mustCall(t, session, "perses_create_dashboard", map[string]any{"project": "myproject", "dashboard": asObject, "include_dashboard": true})
		if result["dashboard"] == nil {
			t.Errorf("unexpected nil")
		}
	})
	t.Run("empty metadata.project is filled before validation", func(t *testing.T) {
		fake := newFakePerses()
		session := newSession(t, fake)
		mustCall(t, session, "perses_create_dashboard", map[string]any{"project": "myproject", "dashboard": asObject, "dry_run": true})
		equal(t, "myproject", fake.validated["metadata"].(map[string]any)["project"], "project variables are only resolved with metadata.project")
	})
	t.Run("project mismatch", func(t *testing.T) {
		fake := newFakePerses()
		session := newSession(t, fake)
		other := strings.Replace(validDashboard, `"name": "cpu"`, `"name": "cpu", "project": "other"`, 1)
		for _, dryRun := range []bool{false, true} {
			text := mustFail(t, session, "perses_create_dashboard", map[string]any{"project": "myproject", "dashboard": object(t, other), "dry_run": dryRun})
			equal(t, "project parameter value 'myproject' does not match provided metadata value 'other'", text)
		}
		equal(t, 0, fake.Count(http.MethodPost, "/api/validate/dashboards"))
		empty(t, fake.dashboards)
	})
	t.Run("project differing only by case is accepted", func(t *testing.T) {
		fake := newFakePerses()
		session := newSession(t, fake)
		upper := strings.Replace(validDashboard, `"name": "cpu"`, `"name": "cpu", "project": "MyProject"`, 1)
		result := mustCall(t, session, "perses_create_dashboard", map[string]any{"project": "myproject", "dashboard": object(t, upper), "dry_run": true})
		equal(t, "would_create", result["action"])
		equal(t, "MyProject", fake.validated["metadata"].(map[string]any)["project"])
	})
	t.Run("dry run includes the dashboard that would be saved", func(t *testing.T) {
		fake := newFakePerses()
		session := newSession(t, fake)
		result := mustCall(t, session, "perses_create_dashboard", map[string]any{"project": "myproject", "dashboard": asObject, "dry_run": true, "include_dashboard": true})
		if result["dashboard"] == nil {
			t.Fatalf("unexpected nil")
		}
		equal(t, "myproject", result["dashboard"].(map[string]any)["metadata"].(map[string]any)["project"])
		empty(t, fake.dashboards)
	})
	t.Run("dry run saves nothing", func(t *testing.T) {
		fake := newFakePerses()
		session := newSession(t, fake)
		result := mustCall(t, session, "perses_create_dashboard", map[string]any{"project": "myproject", "dashboard": asObject, "dry_run": true})
		equal(t, "would_create", result["action"])
		equal(t, true, result["dryRun"])
		equal(t, "myproject", result["project"], "the dashboard omits metadata.project: the input project is used")
		if !(strings.HasSuffix(result["url"].(string), "/projects/myproject/dashboards/cpu")) {
			t.Errorf("%v: expected true", result["url"])
		}
		empty(t, fake.dashboards)
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
		contains(t, text, `invalid kind: "" for a Dashboard type`)
		empty(t, fake.dashboards)
	})
	t.Run("missing metadata is rejected before any request", func(t *testing.T) {
		fake := newFakePerses()
		session := newSession(t, fake)
		for _, dryRun := range []bool{false, true} {
			text := mustFail(t, session, "perses_create_dashboard", map[string]any{"project": "myproject", "dashboard": withoutMetadata(t), "dry_run": dryRun})
			equal(t, "name cannot be empty", text, "dry_run=%v", dryRun)
		}
		empty(t, fake.Received())
	})
	t.Run("dry run reports dashboard lookup failures", func(t *testing.T) {
		fake := newFakePerses()
		fake.getStatus = http.StatusInternalServerError
		session := newSession(t, fake)
		text := mustFail(t, session, "perses_create_dashboard", map[string]any{"project": "myproject", "dashboard": asObject, "dry_run": true})
		contains(t, text, "error checking whether dashboard 'cpu' exists")
		empty(t, fake.dashboards)
		equal(t, 0, fake.Count(http.MethodPost, "/api/v1/projects/myproject/dashboards"))
	})
	t.Run("conflict", func(t *testing.T) {
		fake := newFakePerses()
		session := newSession(t, fake)
		mustCall(t, session, "perses_create_dashboard", map[string]any{"project": "myproject", "dashboard": asObject})
		text := mustFail(t, session, "perses_create_dashboard", map[string]any{"project": "myproject", "dashboard": asObject})
		contains(t, text, "already exists")
		contains(t, text, "perses_update_dashboard")

		text = mustFail(t, session, "perses_create_dashboard", map[string]any{"project": "myproject", "dashboard": asObject, "dry_run": true})
		contains(t, text, "already exists")
		contains(t, text, "perses_update_dashboard")
		equal(t, 2, fake.Count(http.MethodPost, "/api/v1/projects/myproject/dashboards"), "the dry run must not save")
	})
	t.Run("validation error is the error of the Perses client", func(t *testing.T) {
		fake := newFakePerses()
		session := newSession(t, fake)
		invalid := strings.ReplaceAll(validDashboard, `p1`, `p2`)
		invalid = strings.Replace(invalid, `"mean"`, `"average"`, 1)
		for _, dryRun := range []bool{false, true} {
			text := mustFail(t, session, "perses_create_dashboard", map[string]any{"project": "myproject", "dashboard": object(t, invalid), "dry_run": dryRun})
			contains(t, text, "invalid panel p2: spec.calculation: conflicting values \"mean\" and \"average\"")
			contains(t, text, "StatusCode: 400")
		}
		equal(t, 1, fake.Count(http.MethodPost, "/api/validate/dashboards"), "only the dry run calls the validation API")
		empty(t, fake.dashboards)
	})
	t.Run("decode error is the message of the Perses model", func(t *testing.T) {
		fake := newFakePerses()
		session := newSession(t, fake)
		wrongKind := strings.Replace(validDashboard, `"kind": "Dashboard"`, `"kind": "Datasource"`, 1)
		text := mustFail(t, session, "perses_create_dashboard", map[string]any{"project": "myproject", "dashboard": object(t, wrongKind)})
		equal(t, `invalid kind: "Datasource" for a Dashboard type`, text)
		equal(t, 0, fake.Count(http.MethodPost, "/api/validate/dashboards"))
	})
}

func TestUpdate(t *testing.T) {
	// setup returns a fake Perses storing the dashboard cpu at version 0.
	setup := func(t *testing.T) (*fakePerses, *mcp.ClientSession) {
		fake := newFakePerses()
		session := newSession(t, fake)
		mustCall(t, session, "perses_create_dashboard", map[string]any{"project": "myproject", "dashboard": object(t, validDashboard)})
		return fake, session
	}

	t.Run("missing dashboard", func(t *testing.T) {
		fake, session := setup(t)
		missing := strings.Replace(validDashboard, `"cpu"`, `"other"`, 1)
		for _, extra := range []map[string]any{{}, {"expected_version": 0}, {"dry_run": true}} {
			args := map[string]any{"project": "myproject", "dashboard": object(t, missing)}
			for key, value := range extra {
				args[key] = value
			}
			text := mustFail(t, session, "perses_update_dashboard", args)
			equal(t, "dashboard 'other' does not exist in project 'myproject'", text, "%v", extra)
		}
		notContains(t, fake.dashboards, "other")
		equal(t, 1, fake.Count(http.MethodPut, "/api/v1/projects/myproject/dashboards/other"), "without a version check, the PUT reports the missing dashboard")
	})
	t.Run("missing metadata is rejected before any request", func(t *testing.T) {
		fake, session := setup(t)
		before := len(fake.Received())
		for _, extra := range []map[string]any{{}, {"expected_version": 0}, {"dry_run": true}, {"dry_run": true, "expected_version": 0}} {
			args := map[string]any{"project": "myproject", "dashboard": withoutMetadata(t)}
			for key, value := range extra {
				args[key] = value
			}
			text := mustFail(t, session, "perses_update_dashboard", args)
			equal(t, "name cannot be empty", text, "%v", extra)
		}
		equal(t, before, len(fake.Received()), "the empty name must not reach the URL of a request")
	})
	t.Run("update without expected version does not read", func(t *testing.T) {
		fake, session := setup(t)
		before := fake.Count(http.MethodGet, "/api/v1/projects/myproject/dashboards/cpu")
		result := mustCall(t, session, "perses_update_dashboard", map[string]any{"project": "myproject", "dashboard": object(t, validDashboard)})
		equal(t, "updated", result["action"])
		equal(t, before, fake.Count(http.MethodGet, "/api/v1/projects/myproject/dashboards/cpu"), "an update-only role must be able to update")
	})
	t.Run("version mismatch", func(t *testing.T) {
		fake, session := setup(t)
		text := mustFail(t, session, "perses_update_dashboard", map[string]any{"project": "myproject", "dashboard": object(t, validDashboard), "expected_version": 7})
		contains(t, text, "is at version 0, not 7")
		equal(t, 0, fake.Count(http.MethodPut, "/api/v1/projects/myproject/dashboards/cpu"))
	})
	t.Run("validation error is the error of the Perses client", func(t *testing.T) {
		fake, session := setup(t)
		invalid := strings.ReplaceAll(validDashboard, `p1`, `p2`)
		invalid = strings.Replace(invalid, `"mean"`, `"average"`, 1)
		for _, dryRun := range []bool{false, true} {
			text := mustFail(t, session, "perses_update_dashboard", map[string]any{"project": "myproject", "dashboard": object(t, invalid), "dry_run": dryRun})
			contains(t, text, "invalid panel p2: spec.calculation: conflicting values \"mean\" and \"average\"")
			contains(t, text, "StatusCode: 400")
		}
		equal(t, 1, fake.Count(http.MethodPost, "/api/validate/dashboards"), "only the dry run calls the validation API")
		equal(t, 1, fake.Count(http.MethodPut, "/api/v1/projects/myproject/dashboards/cpu"))
		equal(t, float64(0), fake.dashboards["cpu"]["metadata"].(map[string]any)["version"], "the rejected update saves nothing")
	})
	t.Run("update", func(t *testing.T) {
		_, session := setup(t)
		result := mustCall(t, session, "perses_update_dashboard", map[string]any{"project": "myproject", "dashboard": object(t, validDashboard), "expected_version": 0})
		equal(t, "updated", result["action"])
		equal(t, float64(1), result["version"])
	})
	t.Run("project mismatch", func(t *testing.T) {
		fake, session := setup(t)
		other := strings.Replace(validDashboard, `"name": "cpu"`, `"name": "cpu", "project": "other"`, 1)
		before := fake.Count(http.MethodPost, "/api/validate/dashboards")
		text := mustFail(t, session, "perses_update_dashboard", map[string]any{"project": "myproject", "dashboard": object(t, other), "dry_run": true})
		equal(t, "project parameter value 'myproject' does not match provided metadata value 'other'", text)
		equal(t, before, fake.Count(http.MethodPost, "/api/validate/dashboards"))
	})
	t.Run("dry run includes the dashboard that would be saved", func(t *testing.T) {
		fake, session := setup(t)
		result := mustCall(t, session, "perses_update_dashboard", map[string]any{"project": "myproject", "dashboard": object(t, validDashboard), "dry_run": true, "include_dashboard": true})
		if result["dashboard"] == nil {
			t.Fatalf("unexpected nil")
		}
		equal(t, "myproject", result["dashboard"].(map[string]any)["metadata"].(map[string]any)["project"])
		equal(t, 0, fake.Count(http.MethodPut, "/api/v1/projects/myproject/dashboards/cpu"))
	})
	t.Run("dry run saves nothing", func(t *testing.T) {
		fake := newFakePerses()
		session := newSession(t, fake)
		mustCall(t, session, "perses_create_dashboard", map[string]any{"project": "myproject", "dashboard": object(t, validDashboard)})
		before, err := json.Marshal(fake.dashboards["cpu"])
		if err != nil {
			t.Fatal(err)
		}

		renamed := strings.Replace(validDashboard, `"name": "CPU"`, `"name": "Renamed"`, 1)
		result := mustCall(t, session, "perses_update_dashboard", map[string]any{"project": "myproject", "dashboard": object(t, renamed), "dry_run": true, "expected_version": 0})
		equal(t, "would_update", result["action"])
		equal(t, true, result["dryRun"])
		equal(t, 0, fake.Count(http.MethodPut, "/api/v1/projects/myproject/dashboards/cpu"))
		after, err := json.Marshal(fake.dashboards["cpu"])
		if err != nil {
			t.Fatal(err)
		}
		equal(t, string(before), string(after))
		equal(t, float64(0), fake.dashboards["cpu"]["metadata"].(map[string]any)["version"])
	})
}

func TestGetAndList(t *testing.T) {
	fake := newFakePerses()
	session := newSession(t, fake)
	mustCall(t, session, "perses_create_dashboard", map[string]any{"project": "myproject", "dashboard": object(t, validDashboard)})

	full := mustCall(t, session, "perses_get_dashboard_by_name", map[string]any{"project": "myproject", "name": "cpu", "summary": false})
	equal(t, "Dashboard", full["kind"])
	contains(t, full, "spec")

	outline := mustCall(t, session, "perses_get_dashboard_by_name", map[string]any{"project": "myproject", "name": "cpu"})
	if outline["kind"] != nil {
		t.Errorf("expected nil, got %v", outline["kind"])
	}
	notContains(t, outline, "spec")
	panels := outline["panelList"].([]any)
	if !hasLen(t, panels, 1) {
		t.FailNow()
	}
	equal(t, "p1", panels[0].(map[string]any)["key"])
	equal(t, []any{"layout 0: x=0 y=0 w=12 h=6"}, panels[0].(map[string]any)["placement"])
	explicitOutline := mustCall(t, session, "perses_get_dashboard_by_name", map[string]any{"project": "myproject", "name": "cpu", "summary": true})
	equal(t, outline, explicitOutline)

	for _, summary := range []any{nil, true, false} {
		arguments := map[string]any{"project": "myproject", "name": "cpu", "panels": []string{"p1", "nope"}}
		if summary != nil {
			arguments["summary"] = summary
		}
		some := mustCall(t, session, "perses_get_dashboard_by_name", arguments)
		hasLen(t, some["panels"], 1)
		equal(t, []any{"nope"}, some["missing"])
		equal(t, map[string]any{"p1": "CPU"}, some["available"], "missing keys come with the available ones")
		panel := some["panels"].(map[string]any)["p1"].(map[string]any)
		plugin := panel["spec"].(map[string]any)["plugin"].(map[string]any)
		equal(t, "mean", plugin["spec"].(map[string]any)["calculation"])
	}

	found := mustCall(t, session, "perses_get_dashboard_by_name", map[string]any{"project": "myproject", "name": "cpu", "panels": []string{"p1"}})
	notContains(t, found, "available", "available is only returned when a key is missing")

	text := mustFail(t, session, "perses_get_dashboard_by_name", map[string]any{"project": "myproject", "name": "cpu", "panels": []string{""}})
	contains(t, text, "panels")

	text = mustFail(t, session, "perses_get_dashboard_by_name", map[string]any{"project": "myproject", "name": "cpu", "summary": false, "panels": []string{}})
	contains(t, text, "panels")
	notContains(t, text, `"kind"`, "an empty panel selection must not fall back to the full dashboard")

	text = mustFail(t, session, "perses_get_dashboard_by_name", map[string]any{"project": "myproject", "name": "nope"})
	contains(t, text, "does not exist")

	list, isError := callTool(t, session, "perses_list_dashboards", map[string]any{"project": "myproject"})
	if isError {
		t.Fatalf("expected false")
	}
	var listed []map[string]any
	if err := json.Unmarshal([]byte(list), &listed); err != nil {
		t.Fatal(err)
	}
	if !hasLen(t, listed, 1) {
		t.FailNow()
	}
	equal(t, "cpu", listed[0]["name"])
	equal(t, float64(1), listed[0]["panels"])
}

func TestDashboardLinksOmitCredentials(t *testing.T) {
	srv := httptest.NewServer(newFakePerses())
	t.Cleanup(srv.Close)
	base := common.MustParseURL(strings.Replace(srv.URL, "://", "://user:secret@", 1))
	client := apiClient.NewWithClient(&perseshttp.RESTClient{BaseURL: base, Client: srv.Client()})
	session := tooltest.Connect(t, New(client).GetTools())

	created := mustCall(t, session, "perses_create_dashboard", map[string]any{"project": "myproject", "dashboard": object(t, validDashboard)})
	outline := mustCall(t, session, "perses_get_dashboard_by_name", map[string]any{"project": "myproject", "name": "cpu"})
	for _, link := range []any{created["url"], outline["url"]} {
		equal(t, srv.URL+"/projects/myproject/dashboards/cpu", link)
	}
	equal(t, "user:secret", client.RESTClient().BaseURL.User.String())
}

func TestToolDescriptionsStayCompact(t *testing.T) {
	// Every tool description is sent to the model with each tools/list, so the write tools must stay short and leave
	// the dashboard model to perses_get_plugin_schema and the Perses docs. The 1100-character budget is about twice the
	// longest current description: room to grow, but not to turn a description into a manual.
	for _, tool := range New(nil).GetTools() {
		schema, ok := tool.MCPTool.InputSchema.(*jsonschema.Schema)
		if !ok {
			t.Fatalf("expected a JSON Schema for %s", tool.MCPTool.Name)
		}
		if versionSchema := schema.Properties[expectedVersionProperty]; versionSchema != nil {
			contains(t, versionSchema.Description, "best-effort check, not a lock", tool.MCPTool.Name)
		}
		if tool.IsWriteTool && tool.MCPTool.Name != "perses_delete_dashboard" {
			notContains(t, tool.MCPTool.Description, "Model:", tool.MCPTool.Name)
			if len(tool.MCPTool.Description) >= 1100 {
				t.Errorf("%v: expected %v < %v", tool.MCPTool.Name, len(tool.MCPTool.Description), 1100)
			}
		}
	}
}

func TestGetMissingPanelsOnEmptyDashboard(t *testing.T) {
	fake := newFakePerses()
	session := newSession(t, fake)
	empty := strings.Replace(validDashboard, `"panels": {"p1"`, `"unused": {"p1"`, 1)
	var dash map[string]any
	if err := json.Unmarshal([]byte(empty), &dash); err != nil {
		t.Fatal(err)
	}
	spec := dash["spec"].(map[string]any)
	delete(spec, "unused")
	spec["layouts"] = []any{}
	mustCall(t, session, "perses_create_dashboard", map[string]any{"project": "myproject", "dashboard": dash})

	result := mustCall(t, session, "perses_get_dashboard_by_name", map[string]any{"project": "myproject", "name": "cpu", "panels": []string{"p1"}})
	equal(t, []any{"p1"}, result["missing"])
	equal(t, map[string]any{}, result["available"], "an empty dashboard reports that no panel is available")
}

func TestAvailablePanelsWithoutDisplay(t *testing.T) {
	dash := &v1.Dashboard{}
	dash.Spec.Panels = map[string]*dashboardSpec.Panel{"nil": nil, "unnamed": {Kind: "Panel"}}
	equal(t, map[string]string{"nil": "", "unnamed": ""}, availablePanels(dash))
}
