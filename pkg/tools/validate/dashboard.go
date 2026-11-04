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
			"saving it, e.g. to check a draft in read-only mode. Returns {valid: true}, or the error of the Perses " +
			"server. Creating or updating a dashboard runs the same validation.",
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
					Type:        tools.SchemaTypeObject,
					Description: "The dashboard as a JSON object.",
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

func (v *validation) validateDashboard(input ValidateDashboardInput) (*result, error) {
	dashboardObj, err := dashboardutil.DecodeDashboard(input.Dashboard, input.Project)
	if err != nil {
		return invalid(err), nil
	}
	return serverResult(v.client.Validate().Dashboard(dashboardObj))
}
