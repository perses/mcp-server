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
		"perses_delete_dashboard", "perses_set_dashboard_panels", "perses_remove_dashboard_panels",
		"perses_set_dashboard_variables", "perses_remove_dashboard_variables", "perses_set_dashboard_layout",
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
	fake := newFakePerses()
	session := newSession(t, fake)
	mustCall(t, session, "perses_create_dashboard", map[string]any{"project": "myproject", "dashboard": validDashboard})

	t.Run("add panels with automatic placement", func(t *testing.T) {
		panels := map[string]any{
			"p2": object(t, strings.ReplaceAll(statPanel, "%s", "P2")),
			"p3": object(t, strings.ReplaceAll(statPanel, "%s", "P3")),
		}
		result := mustCall(t, session, "perses_set_dashboard_panels", map[string]any{"project": "myproject", "name": "cpu", "panels": panels})
		equal(t, "updated", result["action"])
		equal(t, float64(3), result["panels"])
		changes := result["changes"].([]any)
		equal(t, "added panel p2, placed in layout 0 at x=12 y=0 w=12 h=6", changes[0])
		equal(t, "added panel p3, placed in layout 0 at x=0 y=6 w=12 h=6", changes[1])
		if result["warnings"] != nil {
			t.Errorf("expected nil, got %v", result["warnings"])
		}
		contains(t, fake.stored(t), `"$ref":"#/spec/panels/p3"`)
	})
	t.Run("replace keeps the position", func(t *testing.T) {
		panels := map[string]any{"p2": object(t, strings.ReplaceAll(statPanel, "%s", "P2 bis"))}
		result := mustCall(t, session, "perses_set_dashboard_panels", map[string]any{"project": "myproject", "name": "cpu", "panels": panels})
		equal(t, []any{"replaced panel p2, kept its position"}, result["changes"])
		equal(t, float64(3), result["panels"])
		contains(t, fake.stored(t), "P2 bis")
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
		equal(t, "added panel p4, placed in layout 1 at x=0 y=26 w=24 h=8", changes[0])
		equal(t, "added panel p5, placed in layout 1 at x=6 y=20 w=6 h=6", changes[1])
		equal(t, float64(2), result["layouts"])
	})
	t.Run("panel rejected by the schemas", func(t *testing.T) {
		panels := map[string]any{"p2": object(t, strings.Replace(strings.ReplaceAll(statPanel, "%s", "P2"), "P2\"}}", "average\"}}", 1))}
		text := mustFail(t, session, "perses_set_dashboard_panels", map[string]any{"project": "myproject", "name": "cpu", "panels": panels})
		validation := validationOf(t, text)
		if !hasLen(t, validation.Errors, 1) {
			t.FailNow()
		}
		equal(t, "spec.panels.p2.spec.plugin.spec.calculation", validation.Errors[0].Path)
		contains(t, fake.stored(t), "P2 bis", "the dashboard must not be modified")
	})
	t.Run("query without wrapper is located", func(t *testing.T) {
		panel := strings.Replace(strings.ReplaceAll(statPanel, "%s", "P2"), `}}}}`,
			`}}, "queries": [{"kind": "PrometheusTimeSeriesQuery", "spec": {"query": "up"}}]}}`, 1)
		text := mustFail(t, session, "perses_set_dashboard_panels", map[string]any{"project": "myproject", "name": "cpu", "panels": map[string]any{"p2": object(t, panel)}})
		validation := validationOf(t, text)
		if !hasLen(t, validation.Errors, 1) {
			t.FailNow()
		}
		equal(t, "panels.p2.spec.queries[0]", validation.Errors[0].Path)
		contains(t, validation.Errors[0].Hint, `{kind: "TimeSeriesQuery", spec: {plugin: {kind: "PrometheusTimeSeriesQuery"`)
		contains(t, fake.stored(t), "P2 bis", "the dashboard must not be modified")
	})
	t.Run("invalid key", func(t *testing.T) {
		panels := map[string]any{"bad key": object(t, strings.ReplaceAll(statPanel, "%s", "x"))}
		text := mustFail(t, session, "perses_set_dashboard_panels", map[string]any{"project": "myproject", "name": "cpu", "panels": panels})
		contains(t, text, "the key must match")
	})
	t.Run("panels as a string", func(t *testing.T) {
		panels := `{"p6": ` + strings.ReplaceAll(statPanel, "%s", "P6") + `}`
		result := mustCall(t, session, "perses_set_dashboard_panels", map[string]any{"project": "myproject", "name": "cpu", "panels": panels, "dry_run": true})
		equal(t, "would_update", result["action"])
		notContains(t, fake.stored(t), "p6")
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
		{name: "both omitted", input: map[string]any{}, wantError: "give at least one panel or placement"},
		{
			name:      "both empty",
			input:     map[string]any{"panels": map[string]any{}, "placements": map[string]any{}},
			wantError: "give at least one panel or placement",
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
				contains(t, text, test.wantError)
				equal(t, before, fake.stored(t))
				fake.mu.Lock()
				equal(t, 0, fake.requests["PUT /api/v1/projects/myproject/dashboards/cpu"])
				fake.mu.Unlock()
				if strings.Contains(test.wantError, "at least one") {
					if validationOf(t, text).Valid {
						t.Errorf("expected false")
					}
					contains(t, text, "perses_set_dashboard_layout")
				}
				return
			}
			test.input["dry_run"] = true
			preview := mustCall(t, session, "perses_set_dashboard_panels", test.input)
			equal(t, "would_update", preview["action"])
			equal(t, before, fake.stored(t))
			fake.mu.Lock()
			equal(t, 0, fake.requests["PUT /api/v1/projects/myproject/dashboards/cpu"])
			fake.mu.Unlock()
			delete(test.input, "dry_run")
			result := mustCall(t, session, "perses_set_dashboard_panels", test.input)
			equal(t, preview["changes"], result["changes"])
			var changes []string
			for _, change := range result["changes"].([]any) {
				changes = append(changes, change.(string))
			}
			equal(t, test.wantChanges, changes)
			stored := object(t, fake.stored(t))["spec"].(map[string]any)
			original := object(t, before)["spec"].(map[string]any)
			equal(t, original["panels"].(map[string]any)["p1"], stored["panels"].(map[string]any)["p1"])
			items := stored["layouts"].([]any)[0].(map[string]any)["spec"].(map[string]any)["items"].([]any)
			hasLen(t, items, len(stored["panels"].(map[string]any)))
			for key, raw := range test.input["placements"].(map[string]any) {
				placement := raw.(map[string]any)
				found := false
				for _, rawItem := range items {
					item := rawItem.(map[string]any)
					if item["content"].(map[string]any)["$ref"] != "#/spec/panels/"+key {
						continue
					}
					found = true
					equal(t, fmt.Sprint(placement["x"]), fmt.Sprint(item["x"]))
					equal(t, fmt.Sprint(placement["y"]), fmt.Sprint(item["y"]))
					width, height := 12, 6
					if value, ok := placement["width"]; ok {
						width = value.(int)
					}
					if value, ok := placement["height"]; ok {
						height = value.(int)
					}
					equal(t, fmt.Sprint(width), fmt.Sprint(item["width"]))
					equal(t, fmt.Sprint(height), fmt.Sprint(item["height"]))
				}
				if !found {
					t.Errorf("missing layout item for %s", key)
				}
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
			equal(t, []any{
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
		equal(t, "would_update", result["action"])
		equal(t, []any{"moved panel p1, placed in layout 0 at x=0 y=6 w=12 h=6"}, result["changes"])
		equal(t, before, fake.stored(t))
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
			equal(t, []any{test.wantChange}, result["changes"])
			first := object(t, fake.stored(t))["spec"]
			result = mustCall(t, session, "perses_set_dashboard_panels", test.input)
			equal(t, []any{test.wantChange}, result["changes"], "a repeated call must be a no-op")
			equal(t, first, object(t, fake.stored(t))["spec"])
			layouts := object(t, fake.stored(t))["spec"].(map[string]any)["layouts"].([]any)
			if !hasLen(t, layouts, 2) {
				t.FailNow()
			}
			for i, rawLayout := range layouts {
				for _, rawItem := range rawLayout.(map[string]any)["spec"].(map[string]any)["items"].([]any) {
					item := rawItem.(map[string]any)
					if item["content"].(map[string]any)["$ref"] != "#/spec/panels/p1" {
						continue
					}
					equal(t, test.wantLayout, i)
					for field, want := range test.wantItem {
						equal(t, fmt.Sprint(want), fmt.Sprint(item[field]), field)
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
			equal(t, []any{test.wantChange}, result["changes"])

			layouts := object(t, fake.stored(t))["spec"].(map[string]any)["layouts"].([]any)
			if !hasLen(t, layouts, 2) {
				t.FailNow()
			}
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

			if !hasLen(t, found, 3, "each panel must be placed exactly once: %v", found) {
				t.FailNow()
			}
			contains(t, found, "#/spec/panels/p1@0/0")
			contains(t, found, "#/spec/panels/p3@1")
			item, ok := found["#/spec/panels/p2@"+test.wantLocation]
			if !ok {
				t.Fatalf("p2 not at %s: %v", test.wantLocation, found)
			}
			for field, want := range test.wantItem {
				equal(t, fmt.Sprint(want), fmt.Sprint(item[field]), field)
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
			contains(t, text, `placements.p1: the panel appears in 2 places (layout 0 "Overview", layout 1 "Tabbed" tab 0 "A")`)
			contains(t, text, "perses_set_dashboard_layout")
			equal(t, before, fake.stored(t), "the dashboard must not change")
		})
	}

	t.Run("replace without placement keeps both items", func(t *testing.T) {
		result := mustCall(t, session, "perses_set_dashboard_panels", map[string]any{
			"project": "myproject", "name": "cpu",
			"panels": map[string]any{"p1": json.RawMessage(fmt.Sprintf(statPanel, "CPU bis", "mean"))},
		})
		equal(t, []any{"replaced panel p1, kept its position"}, result["changes"])
		stored, wanted := object(t, fake.stored(t)), object(t, before)
		equal(t, wanted["spec"].(map[string]any)["layouts"], stored["spec"].(map[string]any)["layouts"])
		equal(t, "CPU bis", stored["spec"].(map[string]any)["panels"].(map[string]any)["p1"].(map[string]any)["spec"].(map[string]any)["display"].(map[string]any)["name"])
	})
}

func TestEditToolHints(t *testing.T) {
	d := &dashboard{}
	for _, test := range []struct {
		tool       *tools.Tool
		idempotent bool
	}{
		{d.SetPanels(), true},
		{d.RemovePanels(), true},
		{d.SetVariables(), true},
		{d.RemoveVariables(), true},
		{d.SetLayout(), false},
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
	mustCall(t, session, "perses_create_dashboard", map[string]any{"project": "myproject", "dashboard": validDashboard})

	result := mustCall(t, session, "perses_remove_dashboard_panels", map[string]any{"project": "myproject", "name": "cpu", "panels": []string{"p1", "nope"}})
	equal(t, []any{"removed panel p1 and 1 layout item(s)", "panel nope not found"}, result["changes"])
	equal(t, float64(0), result["panels"])
	notContains(t, fake.stored(t), "#/spec/panels/p1")
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
	equal(t, []any{"added variable env", "added variable inst"}, result["changes"])
	equal(t, float64(2), result["variables"])

	result = mustCall(t, session, "perses_set_dashboard_variables", map[string]any{"project": "myproject", "name": "cpu",
		"variables": []any{object(t, `{"kind": "TextVariable", "spec": {"name": "env", "value": "staging"}}`)}})
	equal(t, []any{"replaced variable env"}, result["changes"])
	equal(t, float64(2), result["variables"])
	contains(t, fake.stored(t), "staging")

	result = mustCall(t, session, "perses_remove_dashboard_variables", map[string]any{"project": "myproject", "name": "cpu", "variables": []string{"env", "nope"}})
	equal(t, []any{"removed variable env", "variable nope not found"}, result["changes"])
	equal(t, float64(1), result["variables"])

	text := mustFail(t, session, "perses_set_dashboard_variables", map[string]any{"project": "myproject", "name": "cpu", "variables": `[{"kind": "TextVariable", "spec": {"value": "x"}}]`})
	contains(t, text, "variables")
}

func TestSetLayout(t *testing.T) {
	fake := newFakePerses()
	session := newSession(t, fake)
	mustCall(t, session, "perses_create_dashboard", map[string]any{"project": "myproject", "dashboard": validDashboard})

	layout := `{"kind": "Grid", "spec": {"display": {"title": "Second"}, "items": []}}`
	result := mustCall(t, session, "perses_set_dashboard_layout", map[string]any{"project": "myproject", "name": "cpu", "layout": layout})
	equal(t, []any{`added layout 1 "Second"`}, result["changes"])

	result = mustCall(t, session, "perses_set_dashboard_layout", map[string]any{"project": "myproject", "name": "cpu", "layout": strings.Replace(layout, "Second", "second", 1)})
	equal(t, []any{`replaced layout 1 "second"`}, result["changes"])
	equal(t, float64(2), result["layouts"])

	result = mustCall(t, session, "perses_set_dashboard_layout", map[string]any{"project": "myproject", "name": "cpu", "index": 0, "layout": layout})
	equal(t, []any{`replaced layout 0 "Second"`}, result["changes"])
	// p1 is no longer displayed.
	if !hasLen(t, result["warnings"], 1) {
		t.FailNow()
	}

	result = mustCall(t, session, "perses_set_dashboard_layout", map[string]any{"project": "myproject", "name": "cpu", "index": 1, "remove": true})
	equal(t, []any{`removed layout 1 "second"`}, result["changes"])
	equal(t, float64(1), result["layouts"])

	text := mustFail(t, session, "perses_set_dashboard_layout", map[string]any{"project": "myproject", "name": "cpu", "index": 5, "layout": layout})
	contains(t, text, "out of range")
	text = mustFail(t, session, "perses_set_dashboard_layout", map[string]any{"project": "myproject", "name": "cpu"})
	contains(t, text, "layout is required")
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
