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
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/perses/mcp-server/pkg/tools"
	"github.com/perses/mcp-server/pkg/tools/resource"
	apiClient "github.com/perses/perses/pkg/client/api"
	"github.com/perses/perses/pkg/model/api"
)

type migrate struct {
	client apiClient.ClientInterface
}

func New(client apiClient.ClientInterface) resource.Resource {
	return &migrate{
		client: client,
	}
}

// Datasource strategy values offered in the elicitation form presented to the user.
const (
	strategyKeepReferences = "keep-hard-coded-references"
	strategyPersesDefault  = "use-perses-default-datasource"
)

type MigrateDashboardInput struct {
	GrafanaDashboard string            `json:"grafanaDashboard" jsonschema:"Grafana dashboard JSON as a string"`
	Input            map[string]string `json:"input,omitempty" jsonschema:"Grafana input values used to resolve __inputs placeholders (e.g. DS_PROMETHEUS=my-datasource)"`
}

func (m *migrate) GetTools() []*tools.Tool {
	return []*tools.Tool{
		m.Migrate(),
	}
}

// useDefaultDatasourceElicitID identifies the input request/response used to ask the
// user whether the default datasource should be used during migration.
const useDefaultDatasourceElicitID = "useDefaultDatasource"

func (m *migrate) Migrate() *tools.Tool {
	tool := &mcp.Tool{
		Name: "perses_migrate_dashboard",
		Description: "Convert a Grafana dashboard into a native Perses dashboard. " +
			"Use this tool to translate exported Grafana dashboard JSON into the Perses dashboard model, " +
			"The result is returned as JSON only and is NOT saved to Perses; " +
			"use the dashboard create tool to persist it.",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Migrates a Grafana dashboard to the Perses format",
			ReadOnlyHint:    true,
			DestructiveHint: new(false),
			IdempotentHint:  true,
			OpenWorldHint:   new(false),
		},
		InputSchema: &jsonschema.Schema{
			Type: tools.SchemaTypeObject,
			Properties: map[string]*jsonschema.Schema{
				"grafanaDashboard": {
					Type:        tools.SchemaTypeString,
					Description: "Grafana dashboard JSON as a string",
				},
				"input": {
					Type:        tools.SchemaTypeObject,
					Description: "Grafana input values used to resolve __inputs placeholders (e.g. DS_PROMETHEUS=my-datasource)",
					AdditionalProperties: &jsonschema.Schema{
						Type: tools.SchemaTypeString,
					},
				},
			},
			Required: []string{"grafanaDashboard"},
		},
	}

	handler := func(_ context.Context, req *mcp.CallToolRequest, input MigrateDashboardInput) (*mcp.CallToolResult, any, error) { //nolint:unparam
		if input.GrafanaDashboard == "" {
			return nil, nil, fmt.Errorf("grafana dashboard JSON cannot be empty")
		}

		// When no human can be asked, this stays false: preserving the original datasource
		// references is the lossless outcome.
		useDefaultDatasource, chosenByUser := false, false
		switch {
		// The elicitation response must be checked before anything else: the SDK replays the
		// original Arguments on the multi round-trip retry, so an argument-first branch would
		// re-prompt the user on every retry until the retry budget is exhausted.
		case hasUseDefaultDatasourceResponse(req):
			useDefaultDatasource, chosenByUser = resolveUseDefaultDatasourceResponse(req), true

		// Nothing the caller sends can skip this prompt: the user always decides.
		case clientSupportsFormElicitation(req):
			return &mcp.CallToolResult{
				InputRequests: mcp.InputRequestMap{
					useDefaultDatasourceElicitID: useDefaultDatasourceElicitParams(),
				},
			}, nil, nil
		}

		persesDashboard, err := m.client.Migrate(&api.Migrate{
			Input:                input.Input,
			GrafanaDashboard:     json.RawMessage(input.GrafanaDashboard),
			UseDefaultDatasource: useDefaultDatasource,
		})
		if err != nil {
			return nil, nil, fmt.Errorf("error migrating grafana dashboard: %w", err)
		}

		text, err := json.Marshal(persesDashboard)
		if err != nil {
			return nil, nil, fmt.Errorf("error marshalling migrated dashboard: %w", err)
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: datasourceStrategyNote(useDefaultDatasource, chosenByUser),
				},
				&mcp.TextContent{
					Text: string(text),
				},
			},
		}, nil, nil
	}

	return &tools.Tool{
		MCPTool:      tool,
		IsWriteTool:  false,
		ResourceType: tools.MigrateResource,
		RegisterWith: func(server *mcp.Server) { mcp.AddTool(server, tool, handler) },
	}
}

func clientSupportsFormElicitation(req *mcp.CallToolRequest) bool {
	if req == nil || req.Session == nil {
		return false
	}
	params := req.Session.InitializeParams()
	if params == nil {
		return false
	}
	return supportsFormElicitation(params.Capabilities)
}

// supportsFormElicitation reports whether the advertised capabilities include form
// elicitation. A client declaring neither mode predates the form/URL split and is assumed
// to support forms.
func supportsFormElicitation(capabilities *mcp.ClientCapabilities) bool {
	if capabilities == nil || capabilities.Elicitation == nil {
		return false
	}
	elicitation := capabilities.Elicitation
	return elicitation.Form != nil || elicitation.URL == nil
}

// datasourceStrategyNote returns a short human-readable note describing which datasource
// strategy was applied to the migrated dashboard, so the caller understands the outcome
// without diffing the JSON. It also records who made the choice, so a strategy applied
// without asking the user is visible rather than silent.
func datasourceStrategyNote(useDefaultDatasource, chosenByUser bool) string {
	strategy := "preserved the original datasource references from the Grafana dashboard"
	if useDefaultDatasource {
		strategy = "replaced all panel datasources with the default Perses datasource"
	}
	if chosenByUser {
		return fmt.Sprintf("Datasource strategy: %s (chosen by the user).", strategy)
	}
	return fmt.Sprintf("Datasource strategy: %s (server configuration or caller-provided; the user was not asked).", strategy)
}

// useDefaultDatasourceElicitParams builds the elicitation request asking the user
// whether the default Perses datasource should be used for all panels. The user is the
// only one who can answer: there is deliberately no tool argument that pre-empts this.
func useDefaultDatasourceElicitParams() *mcp.ElicitParams {
	return &mcp.ElicitParams{
		Mode:    "form",
		Message: "How should panel datasources be handled in the migrated dashboard?",
		RequestedSchema: &jsonschema.Schema{
			Type: tools.SchemaTypeObject,
			Properties: map[string]*jsonschema.Schema{
				useDefaultDatasourceElicitID: {
					Type:        tools.SchemaTypeString,
					Title:       "Datasource strategy",
					Description: "This choice is applied to all panels in the migrated dashboard.",
					Enum:        []any{strategyKeepReferences, strategyPersesDefault},
					Extra: map[string]any{
						"enumNames": []any{
							"Preserve the original hard-coded datasource references from Grafana",
							"Replace all original hard-coded datasource references with the Perses default datasource",
						},
					},
				},
			},
		},
	}
}

// hasUseDefaultDatasourceResponse reports whether the request carries the response to
// the default-datasource elicitation (i.e. the client fulfilled the input request).
func hasUseDefaultDatasourceResponse(req *mcp.CallToolRequest) bool {
	if req == nil || req.Params == nil {
		return false
	}
	_, ok := req.Params.InputResponses[useDefaultDatasourceElicitID]
	return ok
}

// resolveUseDefaultDatasourceResponse extracts the user's answer from the elicitation
// response. It returns false when the user declined or cancelled, or when the response
// is missing the expected value, so migration preserves datasource references.
//
// Both the current string form and the legacy boolean form are accepted, so clients that
// answered the previous boolean schema keep working.
func resolveUseDefaultDatasourceResponse(req *mcp.CallToolRequest) bool {
	response, ok := req.Params.InputResponses[useDefaultDatasourceElicitID].(*mcp.ElicitResult)
	if !ok || response == nil || response.Action != "accept" {
		return false
	}
	switch value := response.Content[useDefaultDatasourceElicitID].(type) {
	case string:
		return value == strategyPersesDefault
	case bool:
		return value
	default:
		return false
	}
}

// List is not required for migrate
func (m *migrate) List() *tools.Tool {
	return nil
}

// Get is not required for migrate
func (m *migrate) Get() *tools.Tool {
	return nil
}

// Create is not required for migrate
func (m *migrate) Create() *tools.Tool {
	return nil
}

// Update is not required for migrate
func (m *migrate) Update() *tools.Tool {
	return nil
}

// Delete is not required for migrate
func (m *migrate) Delete() *tools.Tool {
	return nil
}
