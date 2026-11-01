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
	v1 "github.com/perses/perses/pkg/model/api/v1"
)

var variableKinds = resourceKinds{subject: "variable", project: string(v1.KindVariable), global: string(v1.KindGlobalVariable)}

type ValidateVariableInput struct {
	Variable json.RawMessage `json:"variable"`
	Scope    string          `json:"scope,omitempty"`
	Project  string          `json:"project,omitempty"`
}

func (v *validation) Variable() *tools.Tool {
	tool := &mcp.Tool{
		Name: "perses_validate_variable",
		Description: "Validate a project or global variable against the plugin schemas without saving it. Returns " +
			"{valid, errors}: each error has the JSON path of the field, a message and often a hint.",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Validates a variable against Perses schemas",
			ReadOnlyHint:    true,
			DestructiveHint: new(false),
			IdempotentHint:  true,
			OpenWorldHint:   new(false),
		},
		InputSchema: &jsonschema.Schema{
			Type: tools.SchemaTypeObject,
			Properties: map[string]*jsonschema.Schema{
				"variable": {
					Types:       []string{tools.SchemaTypeObject, tools.SchemaTypeString},
					Description: "Variable to validate, as a JSON object (preferred) or a JSON string.",
				},
				"scope":                       scopeProperty(variableKinds),
				string(tools.ProjectResource): projectProperty("Project of a project variable; fills metadata.project when omitted."),
			},
			Required: []string{"variable"},
		},
	}

	handler := func(_ context.Context, _ *mcp.CallToolRequest, input ValidateVariableInput) (*mcp.CallToolResult, any, error) { //nolint:unparam
		result, err := v.validateVariable(input)
		if err != nil {
			return nil, nil, err
		}
		text, err := writeResult(result)
		if err != nil {
			return nil, nil, err
		}
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: text}}}, nil, nil
	}

	return &tools.Tool{
		MCPTool:      tool,
		IsWriteTool:  false,
		ResourceType: tools.ValidateResource,
		RegisterWith: func(server *mcp.Server) { mcp.AddTool(server, tool, handler) },
	}
}

func (v *validation) validateVariable(input ValidateVariableInput) (*dashboardutil.Validation, error) {
	if input.Scope != "" && input.Scope != scopeProject && input.Scope != scopeGlobal {
		return nil, fmt.Errorf("invalid scope %q: use %q or %q", input.Scope, scopeProject, scopeGlobal)
	}
	decoded, issues := decodeResource(input.Variable, "variable", variableKinds, input.Scope, input.Project)
	if len(issues) > 0 {
		return dashboardutil.NewValidation(issues, nil), nil
	}
	var err error
	if decoded.scope == scopeProject {
		entity := &v1.Variable{}
		if issues := unmarshalResource(decoded.document, entity); len(issues) > 0 {
			return dashboardutil.NewValidation(issues, nil), nil
		}
		err = v.client.Validate().Variable(entity)
	} else {
		entity := &v1.GlobalVariable{}
		if issues := unmarshalResource(decoded.document, entity); len(issues) > 0 {
			return dashboardutil.NewValidation(issues, nil), nil
		}
		err = v.client.Validate().GlobalVariable(entity)
	}
	if err != nil {
		message, ok := dashboardutil.IsBadRequest(err)
		if !ok {
			return nil, fmt.Errorf("error validating the %s variable: %w", decoded.scope, err)
		}
		return dashboardutil.NewValidation(dashboardutil.ResourceIssuesFromServerMessage(message, "spec.spec.plugin",
			"use perses_get_plugin_schema with the variable plugin kind to see the allowed fields"), nil), nil
	}
	return dashboardutil.NewValidation(nil, nil), nil
}
