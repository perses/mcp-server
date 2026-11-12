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
	"maps"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/perses/mcp-server/pkg/tools"
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

const statPanel = `{"kind": "Panel", "spec": {"display": {"name": "%s"}, "plugin": {"kind": "StatChart", "spec": {"calculation": "%s"}}}}`

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

// stored returns the stored dashboard re-encoded as JSON, for assertions.
func (f *fakePerses) stored(t *testing.T) string {
	const name = "cpu"
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	dash, ok := f.dashboards[name]
	if !ok {
		t.Fatalf("dashboard %s is not stored", name)
	}
	text, err := json.Marshal(dash)
	if err != nil {
		t.Fatal(err)
	}
	return string(text)
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
		"perses_delete_dashboard", "perses_set_dashboard_panels", "perses_set_dashboard_variables",
		"perses_set_dashboard_layout",
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

func TestSetPanels(t *testing.T) {
	// setup returns a fake Perses storing the dashboard cpu with the panels p1, p2 and p3 in the group "Row".
	setup := func(t *testing.T) (*fakePerses, *mcp.ClientSession) {
		fake := newFakePerses()
		session := newSession(t, fake)
		mustCall(t, session, "perses_create_dashboard", map[string]any{"project": "myproject", "dashboard": object(t, validDashboard)})
		panels := map[string]any{
			"p2": object(t, strings.ReplaceAll(statPanel, "%s", "P2")),
			"p3": object(t, strings.ReplaceAll(statPanel, "%s", "P3")),
		}
		mustCall(t, session, "perses_set_dashboard_panels", map[string]any{"project": "myproject", "name": "cpu", "panels": panels})
		return fake, session
	}

	t.Run("add panels to the last group", func(t *testing.T) {
		fake := newFakePerses()
		session := newSession(t, fake)
		mustCall(t, session, "perses_create_dashboard", map[string]any{"project": "myproject", "dashboard": object(t, validDashboard)})
		panels := map[string]any{
			"p2": object(t, strings.ReplaceAll(statPanel, "%s", "P2")),
			"p3": object(t, strings.ReplaceAll(statPanel, "%s", "P3")),
		}
		result := mustCall(t, session, "perses_set_dashboard_panels", map[string]any{"project": "myproject", "name": "cpu", "panels": panels})
		equal(t, "updated", result["action"])
		equal(t, float64(3), result["panels"])
		equal(t, []any{"added panel p2", "added panel p3", `group 0 "Row": p1 x=0 y=0, p2 x=12 y=0, p3 x=0 y=6 (w=12 h=6)`}, result["changes"])
		contains(t, fake.stored(t), `"$ref":"#/spec/panels/p3"`)
	})
	t.Run("replace keeps the position", func(t *testing.T) {
		fake, session := setup(t)
		panels := map[string]any{"p2": object(t, strings.ReplaceAll(statPanel, "%s", "P2 bis"))}
		result := mustCall(t, session, "perses_set_dashboard_panels", map[string]any{"project": "myproject", "name": "cpu", "panels": panels})
		equal(t, []any{"replaced panel p2, kept its position"}, result["changes"])
		equal(t, float64(3), result["panels"])
		contains(t, fake.stored(t), "P2 bis")
	})
	t.Run("new group with the SDK defaults", func(t *testing.T) {
		_, session := setup(t)
		panels := map[string]any{
			"p4": object(t, strings.ReplaceAll(statPanel, "%s", "P4")),
			"p5": object(t, strings.ReplaceAll(statPanel, "%s", "P5")),
		}
		result := mustCall(t, session, "perses_set_dashboard_panels", map[string]any{"project": "myproject", "name": "cpu", "panels": panels, "group": "Details"})
		equal(t, []any{"added panel p4", "added panel p5", `group 1 "Details": p4 x=0 y=0, p5 x=12 y=0 (w=12 h=8)`}, result["changes"])
		equal(t, float64(2), result["layouts"])
	})
	t.Run("panel rejected by the schemas", func(t *testing.T) {
		fake, session := setup(t)
		before := fake.stored(t)
		panels := map[string]any{"p2": object(t, strings.Replace(strings.ReplaceAll(statPanel, "%s", "P2"), "P2\"}}", "average\"}}", 1))}
		text := mustFail(t, session, "perses_set_dashboard_panels", map[string]any{"project": "myproject", "name": "cpu", "panels": panels})
		contains(t, text, `invalid panel p2: spec.calculation: conflicting values "mean" and "average"`)
		equal(t, before, fake.stored(t), "the dashboard must not be modified")
	})
	t.Run("decode error is the message of the Perses model", func(t *testing.T) {
		fake, session := setup(t)
		panels := map[string]any{"p6": map[string]any{"kind": "Panel", "spec": map[string]any{"display": "P6"}}}
		text := mustFail(t, session, "perses_set_dashboard_panels", map[string]any{"project": "myproject", "name": "cpu", "panels": panels})
		contains(t, text, `panels."p6": `)
		contains(t, text, "cannot unmarshal string")
		notContains(t, fake.stored(t), "p6")
	})
}

func TestSetPanelsDecodeErrorsNameThePanel(t *testing.T) {
	fake := newFakePerses()
	session := newSession(t, fake)
	mustCall(t, session, "perses_create_dashboard", map[string]any{"project": "myproject", "dashboard": object(t, validDashboard)})
	before := fake.stored(t)

	emptyKind := object(t, strings.ReplaceAll(statPanel, "%s", "B"))
	emptyKind["spec"].(map[string]any)["queries"] = []any{map[string]any{"kind": "TimeSeriesQuery", "spec": map[string]any{"plugin": map[string]any{"kind": "", "spec": map[string]any{}}}}}
	var direct dashboardSpec.Panel
	raw, err := json.Marshal(emptyKind)
	if err != nil {
		t.Fatal(err)
	}
	perses := json.Unmarshal(raw, &direct)
	if perses == nil {
		t.Fatal("the Perses model must reject a query plugin with an empty kind")
	}
	panels := map[string]any{
		"a": object(t, strings.ReplaceAll(statPanel, "%s", "A")),
		"b": emptyKind,
		"c": object(t, strings.ReplaceAll(statPanel, "%s", "C")),
	}
	text := mustFail(t, session, "perses_set_dashboard_panels", map[string]any{"project": "myproject", "name": "cpu", "panels": panels})
	equal(t, `panels."b": `+perses.Error(), text, "the prefix names the panel and the message of the Perses model is unchanged")

	text = mustFail(t, session, "perses_set_dashboard_panels", map[string]any{"project": "myproject", "name": "cpu", "panels": map[string]any{"a": nil}})
	equal(t, `panels."a" is null: give a panel object, or list the key in remove to delete it`, text)
	equal(t, before, fake.stored(t))
}

func TestSetPanelsCreatesATitledGroup(t *testing.T) {
	fake := newFakePerses()
	session := newSession(t, fake)
	dash := object(t, validDashboard)
	dash["spec"].(map[string]any)["layouts"] = []any{}
	mustCall(t, session, "perses_create_dashboard", map[string]any{"project": "myproject", "dashboard": dash})

	panels := map[string]any{"p2": object(t, strings.ReplaceAll(statPanel, "%s", "P2"))}
	result := mustCall(t, session, "perses_set_dashboard_panels", map[string]any{"project": "myproject", "name": "cpu", "panels": panels})
	equal(t, []any{"added panel p2", `created group 0 "Panel Group": the dashboard had no Grid group`, `group 0 "Panel Group": p2 x=0 y=0 (w=12 h=8)`}, result["changes"])
	contains(t, fake.stored(t), `"display":{"title":"Panel Group"}`)

	// set_dashboard_layout can then target the group.
	result = mustCall(t, session, "perses_set_dashboard_layout", map[string]any{"project": "myproject", "name": "cpu", "title": "Panel Group", "panels": []string{"p1", "p2"}})
	equal(t, []any{`group 0 "Panel Group": p1 x=0 y=0, p2 x=12 y=0 (w=12 h=8)`}, result["changes"])
	equal(t, float64(1), result["layouts"])
	result = mustCall(t, session, "perses_set_dashboard_layout", map[string]any{"project": "myproject", "name": "cpu", "title": "Panel Group", "remove": true})
	equal(t, float64(0), result["layouts"])
}

func TestRemovalLaysOutSDKGroupsAgain(t *testing.T) {
	// setup returns a session on the dashboard cpu with p1, p2 and p3 laid out like the SDK in "Row" and o1 and o2 in
	// "Other".
	setup := func(t *testing.T) (*fakePerses, *mcp.ClientSession) {
		fake := newFakePerses()
		session := newSession(t, fake)
		mustCall(t, session, "perses_create_dashboard", map[string]any{"project": "myproject", "dashboard": object(t, validDashboard)})
		panels := map[string]any{}
		for _, key := range []string{"p2", "p3"} {
			panels[key] = object(t, strings.ReplaceAll(statPanel, "%s", key))
		}
		mustCall(t, session, "perses_set_dashboard_panels", map[string]any{"project": "myproject", "name": "cpu", "panels": panels})
		panels = map[string]any{}
		for _, key := range []string{"o1", "o2"} {
			panels[key] = object(t, strings.ReplaceAll(statPanel, "%s", key))
		}
		mustCall(t, session, "perses_set_dashboard_panels", map[string]any{"project": "myproject", "name": "cpu", "panels": panels, "group": "Other"})
		return fake, session
	}

	t.Run("remove and add in one call", func(t *testing.T) {
		_, session := setup(t)
		panels := map[string]any{"p4": object(t, strings.ReplaceAll(statPanel, "%s", "P4"))}
		result := mustCall(t, session, "perses_set_dashboard_panels", map[string]any{"project": "myproject", "name": "cpu", "panels": panels, "remove": []string{"p1"}, "group": "Row"})
		equal(t, []any{
			"removed panel p1 and 1 layout item(s)",
			`group 0 "Row" laid out again: p2 x=0 y=0, p3 x=12 y=0 (w=12 h=6)`,
			"added panel p4",
			`group 0 "Row": p2 x=0 y=0, p3 x=12 y=0, p4 x=0 y=6 (w=12 h=6)`,
		}, result["changes"])
	})
	t.Run("removing the last panel of a group moves nothing", func(t *testing.T) {
		_, session := setup(t)
		result := mustCall(t, session, "perses_set_dashboard_panels", map[string]any{"project": "myproject", "name": "cpu", "remove": []string{"p3"}})
		equal(t, []any{"removed panel p3 and 1 layout item(s)"}, result["changes"])
	})
	t.Run("a group arranged by hand keeps its holes", func(t *testing.T) {
		fake, session := setup(t)
		stored := object(t, fake.stored(t))
		items := stored["spec"].(map[string]any)["layouts"].([]any)[0].(map[string]any)["spec"].(map[string]any)["items"].([]any)
		items[2].(map[string]any)["y"] = float64(20)
		mustCall(t, session, "perses_update_dashboard", map[string]any{"project": "myproject", "dashboard": stored})

		result := mustCall(t, session, "perses_set_dashboard_panels", map[string]any{"project": "myproject", "name": "cpu", "remove": []string{"p1"}})
		equal(t, []any{"removed panel p1 and 1 layout item(s)"}, result["changes"])
		contains(t, fake.stored(t), `{"content":{"$ref":"#/spec/panels/p2"},"height":6,"width":12,"x":12,"y":0}`)
	})
	t.Run("set_dashboard_layout lays out again the groups it takes panels from", func(t *testing.T) {
		fake, session := setup(t)
		stored := object(t, fake.stored(t))
		items := stored["spec"].(map[string]any)["layouts"].([]any)[0].(map[string]any)["spec"].(map[string]any)["items"].([]any)
		items[1].(map[string]any)["repeatVariable"] = map[string]any{"value": "instance"}
		mustCall(t, session, "perses_update_dashboard", map[string]any{"project": "myproject", "dashboard": stored})

		result := mustCall(t, session, "perses_set_dashboard_layout", map[string]any{"project": "myproject", "name": "cpu", "title": "Other", "panels": []string{"o1", "o2", "p1"}})
		equal(t, []any{
			"moved 1 layout item(s) out of other Grid groups",
			`group 0 "Row" laid out again: p2 x=0 y=0, p3 x=12 y=0 (w=12 h=6)`,
			`group 1 "Other": o1 x=0 y=0, o2 x=12 y=0, p1 x=0 y=8 (w=12 h=8)`,
		}, result["changes"])
		contains(t, fake.stored(t), `{"content":{"$ref":"#/spec/panels/p2"},"height":6,"repeatVariable":{"value":"instance"},"width":12,"x":0,"y":0}`,
			"the group laid out again keeps the repeat settings")
	})
}

func TestEditsAreSerialized(t *testing.T) {
	fake := newFakePerses()
	session := newSession(t, fake)
	mustCall(t, session, "perses_create_dashboard", map[string]any{"project": "myproject", "dashboard": object(t, validDashboard)})

	calls := []struct {
		tool string
		args map[string]any
	}{
		{"perses_set_dashboard_panels", map[string]any{"panels": map[string]any{"p2": object(t, strings.ReplaceAll(statPanel, "%s", "P2"))}}},
		{"perses_set_dashboard_variables", map[string]any{"variables": []any{object(t, `{"kind": "TextVariable", "spec": {"name": "env", "value": "prod"}}`)}}},
	}
	results := make([]*mcp.CallToolResult, len(calls))
	errs := make([]error, len(calls))
	var wg sync.WaitGroup
	for i, call := range calls {
		call.args["project"], call.args["name"] = "myproject", "cpu"
		wg.Go(func() {
			results[i], errs[i] = session.CallTool(t.Context(), &mcp.CallToolParams{Name: call.tool, Arguments: call.args})
		})
	}
	wg.Wait()
	for i, call := range calls {
		if errs[i] != nil || results[i].IsError {
			t.Fatalf("%s failed: %v %v", call.tool, errs[i], results[i])
		}
	}
	stored := fake.stored(t)
	contains(t, stored, `"$ref":"#/spec/panels/p2"`, "the panel edit must persist")
	contains(t, stored, `"name":"env"`, "the variable edit must persist")
	equal(t, float64(2), object(t, stored)["metadata"].(map[string]any)["version"])
}

func TestEditToolsCheckTheDashboard(t *testing.T) {
	fake := newFakePerses()
	session := newSession(t, fake)
	mustCall(t, session, "perses_create_dashboard", map[string]any{"project": "myproject", "dashboard": object(t, validDashboard)})
	before := fake.stored(t)

	for tool, args := range map[string]map[string]any{
		"perses_set_dashboard_panels":    {"panels": map[string]any{"p2": object(t, strings.ReplaceAll(statPanel, "%s", "P2"))}},
		"perses_set_dashboard_variables": {"variables": []any{object(t, `{"kind": "TextVariable", "spec": {"name": "env", "value": "prod"}}`)}},
		"perses_set_dashboard_layout":    {"title": "Row", "panels": []string{"p1"}},
	} {
		t.Run(tool, func(t *testing.T) {
			withArgs := func(extra map[string]any) map[string]any {
				result := map[string]any{"project": "myproject"}
				maps.Copy(result, args)
				maps.Copy(result, extra)
				return result
			}
			text := mustFail(t, session, tool, withArgs(map[string]any{"name": "cpu", "expected_version": 7}))
			equal(t, "dashboard 'cpu' is at version 0, not 7: it has been modified since you read it; read it again before updating", text)
			text = mustFail(t, session, tool, withArgs(map[string]any{"name": "other"}))
			equal(t, "dashboard 'other' does not exist in project 'myproject'", text)
			equal(t, before, fake.stored(t))
			equal(t, 0, fake.Count(http.MethodPut, "/api/v1/projects/myproject/dashboards/cpu"))
		})
	}
}

func TestSetPanelsAppendsToTheGroup(t *testing.T) {
	panels := map[string]any{
		"p2": object(t, strings.ReplaceAll(statPanel, "%s", "P2")),
		"p3": object(t, strings.ReplaceAll(statPanel, "%s", "P3")),
	}
	t.Run("a group laid out like the SDK is laid out again", func(t *testing.T) {
		fake := newFakePerses()
		session := newSession(t, fake)
		sdk := strings.Replace(validDashboard, `"x": 0, "y": 0, "width": 12, "height": 6`, `"x": 0, "y": 0, "width": 8, "height": 4`, 1)
		mustCall(t, session, "perses_create_dashboard", map[string]any{"project": "myproject", "dashboard": object(t, sdk)})

		result := mustCall(t, session, "perses_set_dashboard_panels", map[string]any{"project": "myproject", "name": "cpu", "panels": panels})
		// The size of the first panel gives 3 panels per line.
		equal(t, []any{"added panel p2", "added panel p3", `group 0 "Row": p1 x=0 y=0, p2 x=8 y=0, p3 x=16 y=0 (w=8 h=4)`}, result["changes"])
	})
	t.Run("a group arranged by hand keeps its positions", func(t *testing.T) {
		fake := newFakePerses()
		session := newSession(t, fake)
		custom := strings.Replace(validDashboard, `"x": 0, "y": 0, "width": 12, "height": 6`, `"x": 6, "y": 3, "width": 8, "height": 4`, 1)
		mustCall(t, session, "perses_create_dashboard", map[string]any{"project": "myproject", "dashboard": object(t, custom)})

		result := mustCall(t, session, "perses_set_dashboard_panels", map[string]any{"project": "myproject", "name": "cpu", "panels": panels})
		// The new panels take the size of the first panel, 3 per line, below its bottom (y=3+4).
		equal(t, []any{
			"added panel p2", "added panel p3",
			`group 0 "Row" is arranged by hand: kept its positions and appended p2, p3 below`,
			`group 0 "Row": p1 x=6 y=3, p2 x=0 y=7, p3 x=8 y=7 (w=8 h=4)`,
		}, result["changes"])
		contains(t, fake.stored(t), `{"content":{"$ref":"#/spec/panels/p1"},"height":4,"width":8,"x":6,"y":3}`)
	})
}

func TestLayoutKeepsRepeatVariables(t *testing.T) {
	fake := newFakePerses()
	session := newSession(t, fake)
	repeated := strings.Replace(validDashboard, `"content": {"$ref": "#/spec/panels/p1"}`,
		`"content": {"$ref": "#/spec/panels/p1"}, "repeatVariable": {"value": "instance"}`, 1)
	mustCall(t, session, "perses_create_dashboard", map[string]any{"project": "myproject", "dashboard": object(t, repeated)})
	const repeat = `"repeatVariable":{"value":"instance"}`

	panels := map[string]any{"p2": object(t, strings.ReplaceAll(statPanel, "%s", "P2"))}
	mustCall(t, session, "perses_set_dashboard_panels", map[string]any{"project": "myproject", "name": "cpu", "panels": panels})
	equal(t, 1, strings.Count(fake.stored(t), repeat))

	mustCall(t, session, "perses_set_dashboard_layout", map[string]any{"project": "myproject", "name": "cpu", "title": "Other", "panels": []string{"p2", "p1"}})
	equal(t, 1, strings.Count(fake.stored(t), repeat), "moving the panel to another group keeps its repeat setting")
}

func TestEditToolHints(t *testing.T) {
	d := &dashboard{}
	for _, test := range []struct {
		tool       *tools.Tool
		idempotent bool
	}{
		{d.SetPanels(), true},
		{d.SetVariables(), true},
		{d.SetLayout(), true},
	} {
		annotations := test.tool.MCPTool.Annotations
		equal(t, test.idempotent, annotations.IdempotentHint, test.tool.MCPTool.Name)
		if !(*annotations.DestructiveHint) {
			t.Errorf("%v: expected true", test.tool.MCPTool.Name)
		}
		if annotations.ReadOnlyHint {
			t.Errorf("%v: expected false", test.tool.MCPTool.Name)
		}
	}
}

func TestRemovePanels(t *testing.T) {
	fake := newFakePerses()
	session := newSession(t, fake)
	mustCall(t, session, "perses_create_dashboard", map[string]any{"project": "myproject", "dashboard": object(t, validDashboard)})

	result := mustCall(t, session, "perses_set_dashboard_panels", map[string]any{"project": "myproject", "name": "cpu", "remove": []string{"p1", "nope"}})
	equal(t, []any{"removed panel p1 and 1 layout item(s)", "panel nope not found"}, result["changes"])
	equal(t, float64(0), result["panels"])
	notContains(t, fake.stored(t), "#/spec/panels/p1")
}

func TestSetPanelsRemoveAndAddInOneCall(t *testing.T) {
	fake := newFakePerses()
	session := newSession(t, fake)
	mustCall(t, session, "perses_create_dashboard", map[string]any{"project": "myproject", "dashboard": object(t, validDashboard)})

	panels := map[string]any{"p2": object(t, strings.ReplaceAll(statPanel, "%s", "P2"))}
	result := mustCall(t, session, "perses_set_dashboard_panels", map[string]any{"project": "myproject", "name": "cpu", "panels": panels, "remove": []string{"p1"}})
	// The removal empties the group, which takes the SDK defaults.
	equal(t, []any{"removed panel p1 and 1 layout item(s)", "added panel p2", `group 0 "Row": p2 x=0 y=0 (w=12 h=8)`}, result["changes"])
	equal(t, float64(1), result["panels"])

	text := mustFail(t, session, "perses_set_dashboard_panels", map[string]any{"project": "myproject", "name": "cpu", "panels": panels, "remove": []string{"p2"}})
	contains(t, text, "either removed or set")
	text = mustFail(t, session, "perses_set_dashboard_panels", map[string]any{"project": "myproject", "name": "cpu"})
	contains(t, text, "give at least one panel to set or one key to remove")
}

func TestSetPanelsRemoveIsAtomic(t *testing.T) {
	fake := newFakePerses()
	session := newSession(t, fake)
	mustCall(t, session, "perses_create_dashboard", map[string]any{"project": "myproject", "dashboard": object(t, validDashboard)})
	before := fake.stored(t)

	invalidPanel := strings.Replace(strings.ReplaceAll(statPanel, "%s", "P2"), "P2\"}}", "average\"}}", 1)
	for _, test := range []struct {
		name      string
		args      map[string]any
		wantError string
	}{
		{
			name:      "removed key also set",
			args:      map[string]any{"remove": []string{"p1"}, "panels": map[string]any{"p1": object(t, strings.ReplaceAll(statPanel, "%s", "P1"))}},
			wantError: "either removed or set",
		},
		{
			name:      "removal with a panel rejected by the schemas",
			args:      map[string]any{"remove": []string{"p1"}, "panels": map[string]any{"p2": object(t, invalidPanel)}},
			wantError: `invalid panel p2: spec.calculation: conflicting values "mean" and "average"`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			args := map[string]any{"project": "myproject", "name": "cpu"}
			for key, value := range test.args {
				args[key] = value
			}
			text := mustFail(t, session, "perses_set_dashboard_panels", args)
			contains(t, text, test.wantError)
			equal(t, before, fake.stored(t), "a failed edit must not remove anything")
		})
	}

	t.Run("dry run removal", func(t *testing.T) {
		result := mustCall(t, session, "perses_set_dashboard_panels", map[string]any{"project": "myproject", "name": "cpu", "remove": []string{"p1"}, "dry_run": true})
		equal(t, "would_update", result["action"])
		equal(t, float64(0), result["panels"])
		equal(t, before, fake.stored(t))
	})
}

func TestVariables(t *testing.T) {
	fake := newFakePerses()
	session := newSession(t, fake)
	mustCall(t, session, "perses_create_dashboard", map[string]any{"project": "myproject", "dashboard": object(t, validDashboard)})

	variables := `[
		{"kind": "TextVariable", "spec": {"name": "env", "value": "prod"}},
		{"kind": "ListVariable", "spec": {"name": "inst", "plugin": {"kind": "PrometheusLabelValuesVariable", "spec": {"labelName": "instance"}}}}
	]`
	result := mustCall(t, session, "perses_set_dashboard_variables", map[string]any{"project": "myproject", "name": "cpu", "variables": json.RawMessage(variables)})
	equal(t, []any{"added variable env", "added variable inst"}, result["changes"])
	equal(t, float64(2), result["variables"])

	result = mustCall(t, session, "perses_set_dashboard_variables", map[string]any{"project": "myproject", "name": "cpu",
		"variables": []any{object(t, `{"kind": "TextVariable", "spec": {"name": "env", "value": "staging"}}`)}})
	equal(t, []any{"replaced variable env"}, result["changes"])
	equal(t, float64(2), result["variables"])
	contains(t, fake.stored(t), "staging")

	result = mustCall(t, session, "perses_set_dashboard_variables", map[string]any{"project": "myproject", "name": "cpu", "remove": []string{"env", "nope"}})
	equal(t, []any{"removed variable env", "variable nope not found"}, result["changes"])
	equal(t, float64(1), result["variables"])

	result = mustCall(t, session, "perses_set_dashboard_variables", map[string]any{"project": "myproject", "name": "cpu", "remove": []string{"inst"},
		"variables": []any{object(t, `{"kind": "TextVariable", "spec": {"name": "environment", "value": "production"}}`)}})
	equal(t, []any{"removed variable inst", "added variable environment"}, result["changes"])
	equal(t, float64(1), result["variables"])

	stored := fake.stored(t)
	text := mustFail(t, session, "perses_set_dashboard_variables", map[string]any{"project": "myproject", "name": "cpu", "remove": []string{"environment"},
		"variables": []any{object(t, `{"kind": "TextVariable", "spec": {"name": "environment", "value": "x"}}`)}})
	contains(t, text, "either removed or set")
	// A removal combined with a variable rejected by the Perses model must not remove anything.
	text = mustFail(t, session, "perses_set_dashboard_variables", map[string]any{"project": "myproject", "name": "cpu", "remove": []string{"environment"},
		"variables": json.RawMessage(`[{"kind": "TextVariable", "spec": {"value": "x"}}]`)})
	equal(t, "variables[0]: name cannot be empty", text)
	text = mustFail(t, session, "perses_set_dashboard_variables", map[string]any{"project": "myproject", "name": "cpu"})
	contains(t, text, "at least one variable")
	text = mustFail(t, session, "perses_set_dashboard_variables", map[string]any{"project": "myproject", "name": "cpu", "variables": []any{}})
	contains(t, text, "at least one variable")
	text = mustFail(t, session, "perses_set_dashboard_variables", map[string]any{"project": "myproject", "name": "cpu", "variables": []any{
		object(t, `{"kind": "TextVariable", "spec": {"name": "dup", "value": "a"}}`),
		object(t, `{"kind": "TextVariable", "spec": {"name": "dup", "value": "b"}}`),
	}})
	equal(t, `variables: "dup" is given twice`, text)
	equal(t, stored, fake.stored(t), "a rejected variable edit must not change the dashboard")

	result = mustCall(t, session, "perses_set_dashboard_variables", map[string]any{"project": "myproject", "name": "cpu", "variables": []any{}, "remove": []string{"environment"}, "dry_run": true})
	equal(t, "would_update", result["action"])
	equal(t, float64(0), result["variables"])
	equal(t, stored, fake.stored(t))

	text = mustFail(t, session, "perses_set_dashboard_variables", map[string]any{"project": "myproject", "name": "cpu", "variables": json.RawMessage(`[{"kind": "TextVariable", "spec": {"value": "x"}}]`)})
	equal(t, "variables[0]: name cannot be empty", text)

	// An error names the variable it comes from and keeps the message of the Perses model.
	text = mustFail(t, session, "perses_set_dashboard_variables", map[string]any{"project": "myproject", "name": "cpu", "variables": json.RawMessage(`[
		{"kind": "TextVariable", "spec": {"name": "a", "value": "x"}},
		{"kind": "TextVariable", "spec": {"name": "b", "value": "x"}},
		{"kind": "ListVariable", "spec": {"name": "c", "sort": "nope", "plugin": {"kind": "PrometheusLabelValuesVariable", "spec": {"labelName": "job"}}}}
	]`)})
	equal(t, `variables[2]: unknown sort method "nope" used`, text)
	equal(t, stored, fake.stored(t))
}

func TestSetLayout(t *testing.T) {
	fake := newFakePerses()
	session := newSession(t, fake)
	mustCall(t, session, "perses_create_dashboard", map[string]any{"project": "myproject", "dashboard": object(t, validDashboard)})
	panels := map[string]any{}
	for _, key := range []string{"p2", "p3", "p4"} {
		panels[key] = object(t, strings.ReplaceAll(statPanel, "%s", key))
	}
	mustCall(t, session, "perses_set_dashboard_panels", map[string]any{"project": "myproject", "name": "cpu", "panels": panels})
	setLayout := func(args map[string]any) map[string]any {
		args["project"], args["name"] = "myproject", "cpu"
		return mustCall(t, session, "perses_set_dashboard_layout", args)
	}

	result := setLayout(map[string]any{"title": "Details", "panels": []string{"p4", "p3"}, "panels_per_line": 3, "panel_height": 6})
	equal(t, []any{"moved 2 layout item(s) out of other Grid groups", `group 1 "Details": p4 x=0 y=0, p3 x=8 y=0 (w=8 h=6)`}, result["changes"])

	result = setLayout(map[string]any{"title": "Row", "panels": []string{"p1", "p2", "p3", "p4"}, "panels_per_line": 3, "panel_height": 6})
	equal(t, []any{"moved 2 layout item(s) out of other Grid groups",
		`group 0 "Row": p1 x=0 y=0, p2 x=8 y=0, p3 x=16 y=0, p4 x=0 y=6 (w=8 h=6)`}, result["changes"])

	// The group keeps its title, panels per line and height; panels left out are no longer displayed.
	result = setLayout(map[string]any{"title": "row", "panels": []string{"p2"}, "collapsed": true})
	equal(t, []any{`group 0 "Row": p2 x=0 y=0 (w=8 h=6)`, "panels no longer displayed: p1, p3, p4"}, result["changes"])
	contains(t, fake.stored(t), `"collapse":{"open":false}`)

	result = setLayout(map[string]any{"title": "Details", "remove": true})
	equal(t, []any{`removed group 1 "Details"`}, result["changes"])
	equal(t, float64(1), result["layouts"])

	for _, test := range []struct {
		name      string
		args      map[string]any
		wantError string
	}{
		{name: "unknown panel", args: map[string]any{"title": "Row", "panels": []string{"nope"}}, wantError: `panel "nope" not found`},
		{name: "no panels", args: map[string]any{"title": "Row"}, wantError: "panels is required"},
		{name: "remove an unknown group", args: map[string]any{"title": "Details", "remove": true}, wantError: "no Grid group is titled"},
		{name: "no title", args: map[string]any{"panels": []string{"p1"}}, wantError: `missing properties: ["title"]`},
		{name: "remove with panels", args: map[string]any{"title": "row", "remove": true, "panels": []string{"p2"}}, wantError: "cannot be given with it"},
		{name: "remove with panel_height", args: map[string]any{"title": "row", "remove": true, "panel_height": 3}, wantError: "cannot be given with it"},
	} {
		t.Run(test.name, func(t *testing.T) {
			test.args["project"], test.args["name"] = "myproject", "cpu"
			contains(t, mustFail(t, session, "perses_set_dashboard_layout", test.args), test.wantError)
		})
	}
}

func TestSetLayoutWithDuplicateTitles(t *testing.T) {
	fake := newFakePerses()
	session := newSession(t, fake)
	dash := object(t, validDashboard)
	layouts := dash["spec"].(map[string]any)["layouts"].([]any)
	dash["spec"].(map[string]any)["layouts"] = append(layouts, object(t, `{"kind": "Grid", "spec": {"display": {"title": "Row"}, "items": []}}`))
	mustCall(t, session, "perses_create_dashboard", map[string]any{"project": "myproject", "dashboard": dash})
	before := fake.stored(t)

	for _, args := range []map[string]any{{"remove": true}, {"panels": []string{"p1"}}} {
		args["project"], args["name"], args["title"] = "myproject", "cpu", "Row"
		text := mustFail(t, session, "perses_set_dashboard_layout", args)
		equal(t, `several Grid groups are titled "Row" (group 0 "Row", group 1 "Row"): rename all but one with perses_update_dashboard to target it`, text)
	}
	text := mustFail(t, session, "perses_set_dashboard_panels", map[string]any{"project": "myproject", "name": "cpu", "group": "Row",
		"panels": map[string]any{"p2": object(t, strings.ReplaceAll(statPanel, "%s", "P2"))}})
	contains(t, text, "several Grid groups are titled")
	equal(t, before, fake.stored(t), "an ambiguous title must not modify the dashboard")
}

func TestSetLayoutRemoveReportsHiddenPanels(t *testing.T) {
	fake := newFakePerses()
	session := newSession(t, fake)
	mustCall(t, session, "perses_create_dashboard", map[string]any{"project": "myproject", "dashboard": object(t, validDashboard)})
	result := mustCall(t, session, "perses_set_dashboard_layout", map[string]any{"project": "myproject", "name": "cpu", "title": "row", "remove": true})
	equal(t, []any{`removed group 0 "Row"`, "panels no longer displayed: p1"}, result["changes"])
	equal(t, float64(1), result["panels"])
}

func TestSetLayoutLeavesTabsUntouched(t *testing.T) {
	fake := newFakePerses()
	session := newSession(t, fake)
	tabs := strings.Replace(validDashboard, `{"kind": "Grid", "spec": {"display": {"title": "Row"}, "items"`,
		`{"kind": "Tabs", "spec": {"tabs": [{"name": "A", "items"`, 1)
	tabs = strings.Replace(tabs, `"#/spec/panels/p1"}}]}}]`, `"#/spec/panels/p1"}}]}]}}]`, 1)
	mustCall(t, session, "perses_create_dashboard", map[string]any{"project": "myproject", "dashboard": object(t, tabs)})

	result := mustCall(t, session, "perses_set_dashboard_layout", map[string]any{"project": "myproject", "name": "cpu", "title": "Grid", "panels": []string{"p1"}})
	equal(t, []any{`group 1 "Grid": p1 x=0 y=0 (w=12 h=8)`, `p1 is also displayed in tab "A" of layout 0`}, result["changes"])
	contains(t, fake.stored(t), `"tabs":[{"items":[{"content":{"$ref":"#/spec/panels/p1"},"height":6,"width":12,"x":0,"y":0}],"name":"A"}]`)
}

func TestSetLayoutPanelHeightIsBounded(t *testing.T) {
	session := newSession(t, newFakePerses())
	mustCall(t, session, "perses_create_dashboard", map[string]any{"project": "myproject", "dashboard": object(t, validDashboard)})

	args := map[string]any{"project": "myproject", "name": "cpu", "title": "Row", "panels": []string{"p1"}, "panel_height": maxPanelHeight + 1}
	result, err := session.CallTool(t.Context(), &mcp.CallToolParams{Name: "perses_set_dashboard_layout", Arguments: args})
	if err == nil && !result.IsError {
		t.Fatalf("a panel_height above %d must be rejected", maxPanelHeight)
	}
	args["panel_height"] = maxPanelHeight
	result2 := mustCall(t, session, "perses_set_dashboard_layout", args)
	equal(t, []any{`group 0 "Row": p1 x=0 y=0 (w=12 h=48)`}, result2["changes"])
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
