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

package validate

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/perses/mcp-server/pkg/tools"
	"github.com/perses/mcp-server/pkg/tools/internal/dashboardutil"
)

type ValidateDashboardInput struct {
	Project   string          `json:"project,omitempty"`
	Dashboard json.RawMessage `json:"dashboard"`
}

func (v *validation) Dashboard() *tools.Tool {
	tool := &mcp.Tool{
		Name: "perses_validate_dashboard",
		Description: "Validate a dashboard against the dashboard and plugin schemas of the Perses server without " +
			"saving it, e.g. to check a draft in read-only mode. Returns {valid, errors, warnings}: each error has the " +
			"JSON path of the offending field (e.g. spec.panels.cpu.spec.plugin.spec.calculation), a message and " +
			"often a hint, so that only that field needs fixing. Warnings flag accepted but likely unintended content, " +
			"e.g. a panel that no layout item displays. Creating or updating a dashboard runs the same validation.",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Validates a dashboard against Perses schemas",
			ReadOnlyHint:    true,
			DestructiveHint: new(false),
			IdempotentHint:  true,
			OpenWorldHint:   new(false),
		},
		InputSchema: &jsonschema.Schema{
			Type: tools.SchemaTypeObject,
			Properties: map[string]*jsonschema.Schema{
				string(tools.ProjectResource): {
					Type:        tools.SchemaTypeString,
					Description: "Project name; fills metadata.project when the dashboard omits it. Optional when metadata.project is already set.",
					MinLength:   new(1),
					MaxLength:   new(75),
					Pattern:     tools.PatternResourceName,
				},
				"dashboard": {
					Types:       []string{tools.SchemaTypeObject, tools.SchemaTypeString},
					Description: "The dashboard as a JSON object (preferred); a JSON document serialized in a string is also accepted.",
				},
			},
			Required: []string{"dashboard"},
		},
	}

	handler := func(_ context.Context, _ *mcp.CallToolRequest, input ValidateDashboardInput) (*mcp.CallToolResult, any, error) { //nolint:unparam
		result, err := v.validateDashboard(input)
		if err != nil {
			return nil, nil, err
		}
		resultJSON, err := dashboardutil.MarshalText(result)
		if err != nil {
			return nil, nil, fmt.Errorf("error marshalling dashboard validation: %w", err)
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: resultJSON}},
		}, nil, nil
	}

	return &tools.Tool{
		MCPTool:      tool,
		IsWriteTool:  false,
		ResourceType: tools.ValidateResource,
		RegisterWith: func(server *mcp.Server) { mcp.AddTool(server, tool, handler) },
	}
}

func (v *validation) validateDashboard(input ValidateDashboardInput) (*dashboardutil.Validation, error) {
	dashboardObj, issues := dashboardutil.Decode(input.Dashboard, input.Project)
	if len(issues) > 0 {
		return dashboardutil.NewValidation(issues, nil), nil
	}
	errs, warnings := dashboardutil.Lint(dashboardObj)
	if err := v.client.Validate().Dashboard(dashboardObj); err != nil {
		message, ok := dashboardutil.IsBadRequest(err)
		if !ok {
			return nil, fmt.Errorf("error validating dashboard in project '%s': %w", input.Project, err)
		}
		errs = append(dashboardutil.IssuesFromServerMessage(message, dashboardObj), errs...)
	}
	return dashboardutil.NewValidation(errs, warnings), nil
}
