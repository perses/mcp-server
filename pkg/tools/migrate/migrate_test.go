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

package migrate

import (
	"strings"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func elicitResult(action string, content map[string]any) *mcp.ElicitResult {
	return &mcp.ElicitResult{Action: action, Content: content}
}

func requestWithResponse(result *mcp.ElicitResult) *mcp.CallToolRequest {
	return &mcp.CallToolRequest{
		Params: &mcp.CallToolParamsRaw{
			InputResponses: mcp.InputResponseMap{useDefaultDatasourceElicitID: result},
		},
	}
}

func TestResolveUseDefaultDatasourceResponse(t *testing.T) {
	testSuites := []struct {
		title    string
		request  *mcp.CallToolRequest
		expected bool
	}{
		{
			title:    "the perses-default value selects the default datasource",
			request:  requestWithResponse(elicitResult("accept", map[string]any{useDefaultDatasourceElicitID: strategyPersesDefault})),
			expected: true,
		},
		{
			title:    "the keep-references value keeps the hard-coded references",
			request:  requestWithResponse(elicitResult("accept", map[string]any{useDefaultDatasourceElicitID: strategyKeepReferences})),
			expected: false,
		},
		{
			// Back-compat: clients that answered the previous boolean schema.
			title:    "legacy boolean true selects the default datasource",
			request:  requestWithResponse(elicitResult("accept", map[string]any{useDefaultDatasourceElicitID: true})),
			expected: true,
		},
		{
			title:    "legacy boolean false keeps the original references",
			request:  requestWithResponse(elicitResult("accept", map[string]any{useDefaultDatasourceElicitID: false})),
			expected: false,
		},
		{
			// The SDK applies the schema Default before this point, but a client returning
			// nil content bypasses that, so the fallback must still hold.
			title:    "accept with nil content falls back to preserve",
			request:  requestWithResponse(elicitResult("accept", nil)),
			expected: false,
		},
		{
			title:    "accept with empty content falls back to preserve",
			request:  requestWithResponse(elicitResult("accept", map[string]any{})),
			expected: false,
		},
		{
			title:    "decline falls back to keeping the references",
			request:  requestWithResponse(elicitResult("decline", map[string]any{useDefaultDatasourceElicitID: strategyPersesDefault})),
			expected: false,
		},
		{
			title:    "cancel falls back to preserve",
			request:  requestWithResponse(elicitResult("cancel", nil)),
			expected: false,
		},
		{
			title:    "unexpected value type falls back to preserve",
			request:  requestWithResponse(elicitResult("accept", map[string]any{useDefaultDatasourceElicitID: 42})),
			expected: false,
		},
	}

	for _, test := range testSuites {
		t.Run(test.title, func(t *testing.T) {
			if got := resolveUseDefaultDatasourceResponse(test.request); got != test.expected {
				t.Errorf("expected %v, got %v", test.expected, got)
			}
		})
	}
}

func TestHasUseDefaultDatasourceResponse(t *testing.T) {
	testSuites := []struct {
		title    string
		request  *mcp.CallToolRequest
		expected bool
	}{
		{
			title:    "nil request",
			request:  nil,
			expected: false,
		},
		{
			title:    "nil params",
			request:  &mcp.CallToolRequest{},
			expected: false,
		},
		{
			title:    "no input responses",
			request:  &mcp.CallToolRequest{Params: &mcp.CallToolParamsRaw{}},
			expected: false,
		},
		{
			title:    "input response present",
			request:  requestWithResponse(elicitResult("accept", nil)),
			expected: true,
		},
	}

	for _, test := range testSuites {
		t.Run(test.title, func(t *testing.T) {
			if got := hasUseDefaultDatasourceResponse(test.request); got != test.expected {
				t.Errorf("expected %v, got %v", test.expected, got)
			}
		})
	}
}

// The SDK replays the original Arguments on a multi round-trip retry. If the handler
// checked anything from the arguments before the elicitation response, every retry would
// issue a fresh prompt until the retry budget is exhausted. This guards that ordering.
func TestElicitationResponseIsCheckedFirst(t *testing.T) {
	request := requestWithResponse(elicitResult("accept", map[string]any{useDefaultDatasourceElicitID: strategyPersesDefault}))

	if !hasUseDefaultDatasourceResponse(request) {
		t.Fatal("expected the elicitation response to be detected")
	}
	if got := resolveUseDefaultDatasourceResponse(request); !got {
		t.Errorf("expected the user's answer to be honoured, got %v", got)
	}
}

func TestClientSupportsFormElicitation(t *testing.T) {
	testSuites := []struct {
		title        string
		capabilities *mcp.ClientCapabilities
		expected     bool
	}{
		{
			title:        "no elicitation capability",
			capabilities: &mcp.ClientCapabilities{},
			expected:     false,
		},
		{
			title:        "form elicitation",
			capabilities: &mcp.ClientCapabilities{Elicitation: &mcp.ElicitationCapabilities{Form: &mcp.FormElicitationCapabilities{}}},
			expected:     true,
		},
		{
			// A URL-only client would reject the form request later with
			// `client does not support "form" elicitation`, so it must not be prompted.
			title:        "url-only elicitation is not enough",
			capabilities: &mcp.ClientCapabilities{Elicitation: &mcp.ElicitationCapabilities{URL: &mcp.URLElicitationCapabilities{}}},
			expected:     false,
		},
		{
			title: "both modes",
			capabilities: &mcp.ClientCapabilities{Elicitation: &mcp.ElicitationCapabilities{
				Form: &mcp.FormElicitationCapabilities{},
				URL:  &mcp.URLElicitationCapabilities{},
			}},
			expected: true,
		},
		{
			// Predates the form/URL split: assumed to support forms.
			title:        "elicitation declared without a mode",
			capabilities: &mcp.ClientCapabilities{Elicitation: &mcp.ElicitationCapabilities{}},
			expected:     true,
		},
	}

	for _, test := range testSuites {
		t.Run(test.title, func(t *testing.T) {
			if got := supportsFormElicitation(test.capabilities); got != test.expected {
				t.Errorf("expected %v, got %v", test.expected, got)
			}
		})
	}
}

func TestClientSupportsFormElicitationNilSafety(t *testing.T) {
	if clientSupportsFormElicitation(nil) {
		t.Error("expected false for a nil request")
	}
	if clientSupportsFormElicitation(&mcp.CallToolRequest{}) {
		t.Error("expected false for a request without a session")
	}
}

// The elicited property must not be Required: the SDK validates accepted content against
// this schema *before* applying defaults, so requiring it would make a client that accepts
// with empty content fail the entire tools/call instead of taking the default.
func TestUseDefaultDatasourceElicitParamsSchema(t *testing.T) {
	params := useDefaultDatasourceElicitParams()

	schema, ok := params.RequestedSchema.(*jsonschema.Schema)
	if !ok {
		t.Fatalf("expected a *jsonschema.Schema, got %T", params.RequestedSchema)
	}
	if len(schema.Required) != 0 {
		t.Errorf("expected no required properties, got %v", schema.Required)
	}

	property := schema.Properties[useDefaultDatasourceElicitID]
	if property == nil {
		t.Fatalf("expected property %q to be present", useDefaultDatasourceElicitID)
	}

	// Enums are only supported on string fields by the SDK.
	if property.Type != "string" {
		t.Errorf("expected a string property, got %q", property.Type)
	}

	// enumNames must have exactly one label per enum value or the client rejects it.
	enumNames, ok := property.Extra["enumNames"].([]any)
	if !ok {
		t.Fatalf("expected enumNames to be an array, got %T", property.Extra["enumNames"])
	}
	if len(enumNames) != len(property.Enum) {
		t.Errorf("expected %d enumNames to match %d enum values, got %d", len(property.Enum), len(property.Enum), len(enumNames))
	}
}

// The form must offer exactly the two real choices. A Default would make the client render
// extra generic "Use default" and "Omit" rows, and Required would make an empty accept fail
// the whole tools/call, so neither may be set.
func TestUseDefaultDatasourceElicitParamsOffersOnlyTheTwoChoices(t *testing.T) {
	params := useDefaultDatasourceElicitParams()
	schema, ok := params.RequestedSchema.(*jsonschema.Schema)
	if !ok {
		t.Fatalf("expected a *jsonschema.Schema, got %T", params.RequestedSchema)
	}
	property := schema.Properties[useDefaultDatasourceElicitID]
	if property == nil {
		t.Fatalf("expected property %q to be present", useDefaultDatasourceElicitID)
	}

	if property.Default != nil {
		t.Errorf("expected no default, got %s", property.Default)
	}
	if len(property.Enum) != 2 {
		t.Errorf("expected exactly 2 enum values, got %d: %v", len(property.Enum), property.Enum)
	}
	if property.Enum[0] != strategyKeepReferences || property.Enum[1] != strategyPersesDefault {
		t.Errorf("expected [%q %q], got %v", strategyKeepReferences, strategyPersesDefault, property.Enum)
	}
}

func TestDatasourceStrategyNote(t *testing.T) {
	testSuites := []struct {
		title                string
		useDefaultDatasource bool
		chosenByUser         bool
		contains             string
	}{
		{
			title:                "user chose the default datasource",
			useDefaultDatasource: true,
			chosenByUser:         true,
			contains:             "chosen by the user",
		},
		{
			title:                "user chose to preserve",
			useDefaultDatasource: false,
			chosenByUser:         true,
			contains:             "chosen by the user",
		},
		{
			// A choice made without asking must be visible rather than silent.
			title:                "applied without asking the user",
			useDefaultDatasource: false,
			chosenByUser:         false,
			contains:             "the user was not asked",
		},
	}

	for _, test := range testSuites {
		t.Run(test.title, func(t *testing.T) {
			note := datasourceStrategyNote(test.useDefaultDatasource, test.chosenByUser)
			if !strings.Contains(note, test.contains) {
				t.Errorf("expected the note to contain %q, got %q", test.contains, note)
			}
		})
	}
}
