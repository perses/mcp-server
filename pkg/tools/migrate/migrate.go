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

type MigrateDashboardInput struct {
	GrafanaDashboard     string            `json:"grafanaDashboard" jsonschema:"Grafana dashboard JSON as a string"`
	Input                map[string]string `json:"input,omitempty" jsonschema:"Grafana input values used to resolve __inputs placeholders (e.g. DS_PROMETHEUS=my-datasource)"`
	UseDefaultDatasource *bool             `json:"useDefaultDatasource,omitempty" jsonschema:"Datasource strategy: true replaces every panel datasource with the default Perses datasource, false preserves the original references. When omitted, the value is decided once via elicitation by asking the user"`
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
		Name:        "perses_migrate_dashboard",
		Description: "Migrate a Grafana dashboard into a native Perses dashboard. The converted Perses dashboard is NOT persisted in Perses.",
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
				"useDefaultDatasource": {
					Type:        tools.SchemaTypeBoolean,
					Description: "Datasource strategy: true replaces every panel datasource with the default Perses datasource, false preserves the original references. When omitted, the value is decided once via elicitation by asking the user",
				},
			},
			Required: []string{"grafanaDashboard"},
		},
	}

	handler := func(_ context.Context, req *mcp.CallToolRequest, input MigrateDashboardInput) (*mcp.CallToolResult, any, error) { //nolint:unparam
		if input.GrafanaDashboard == "" {
			return nil, nil, fmt.Errorf("grafana dashboard JSON cannot be empty")
		}

		useDefaultDatasource := false
		switch {
		case input.UseDefaultDatasource != nil:
			useDefaultDatasource = *input.UseDefaultDatasource
		case hasUseDefaultDatasourceResponse(req):
			useDefaultDatasource = resolveUseDefaultDatasourceResponse(req)
		case clientSupportsElicitation(req):
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
					Text: datasourceStrategyNote(useDefaultDatasource),
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

// clientSupportsElicitation reports whether the connected client advertised elicitation
// support during initialization. When it did not, the migrate tool must not return an
// input request: doing so would fail this read-only tool with "client does not support
// elicitation". Callers can still pass useDefaultDatasource explicitly on such clients.
func clientSupportsElicitation(req *mcp.CallToolRequest) bool {
	if req == nil || req.Session == nil {
		return false
	}
	params := req.Session.InitializeParams()
	if params == nil || params.Capabilities == nil {
		return false
	}
	return params.Capabilities.Elicitation != nil
}

// datasourceStrategyNote returns a short human-readable note describing which datasource
// strategy was applied to the migrated dashboard, so the caller understands the outcome
// without diffing the JSON.
func datasourceStrategyNote(useDefaultDatasource bool) string {
	if useDefaultDatasource {
		return "Datasource strategy: replaced all panel datasources with the default Perses datasource."
	}
	return "Datasource strategy: preserved the original datasource references from the Grafana dashboard."
}

// useDefaultDatasourceElicitParams builds the elicitation request asking the user
// whether the default Perses datasource should be used for all panels.
func useDefaultDatasourceElicitParams() *mcp.ElicitParams {
	return &mcp.ElicitParams{
		Mode:    "form",
		Message: "Choose if the default datasource should be used",
		RequestedSchema: &jsonschema.Schema{
			Type: tools.SchemaTypeObject,
			Properties: map[string]*jsonschema.Schema{
				useDefaultDatasourceElicitID: {
					Type:        tools.SchemaTypeBoolean,
					Description: "Replace all panel datasources with the default Perses datasource? Otherwise, original datasource references from Grafana are preserved. This choice is applied to all panels in the migrated dashboard.",
				},
			},
			Required: []string{useDefaultDatasourceElicitID},
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
// is missing the expected boolean, so migration preserves datasource references.
func resolveUseDefaultDatasourceResponse(req *mcp.CallToolRequest) bool {
	response, ok := req.Params.InputResponses[useDefaultDatasourceElicitID].(*mcp.ElicitResult)
	if !ok || response == nil || response.Action != "accept" {
		return false
	}
	value, ok := response.Content[useDefaultDatasourceElicitID].(bool)
	if !ok {
		return false
	}
	return value
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
