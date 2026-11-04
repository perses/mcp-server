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

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/perses/mcp-server/pkg/tools"
	v1 "github.com/perses/perses/pkg/model/api/v1"
)

var variableKinds = resourceKinds{project: string(v1.KindVariable), global: string(v1.KindGlobalVariable)}

type ValidateVariableInput struct {
	Variable json.RawMessage `json:"variable"`
	Scope    string          `json:"scope,omitempty"`
	Project  string          `json:"project,omitempty"`
}

func (v *validation) Variable() *tools.Tool {
	tool := &mcp.Tool{
		Name: "perses_validate_variable",
		Description: "Validate a project or global variable against the plugin schemas without saving it. Returns " +
			"{valid: true}, or {valid: false, error} with the message of the Perses server.",
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
					Type:        tools.SchemaTypeObject,
					Description: "Variable to validate, as a JSON object.",
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

func (v *validation) validateVariable(input ValidateVariableInput) (*result, error) {
	scope, err := resolveScope(input.Variable, variableKinds, input.Scope, input.Project)
	if err != nil {
		return nil, err
	}
	if scope == scopeProject {
		entity := &v1.Variable{}
		if err := decodeResource(input.Variable, variableKinds, scope, entity); err != nil {
			return invalid(err), nil
		}
		if entity.Metadata.Project == "" {
			entity.Metadata.Project = input.Project
		}
		return serverResult(v.client.Validate().Variable(entity), "error validating the project variable")
	}
	entity := &v1.GlobalVariable{}
	if err := decodeResource(input.Variable, variableKinds, scope, entity); err != nil {
		return invalid(err), nil
	}
	return serverResult(v.client.Validate().GlobalVariable(entity), "error validating the global variable")
}
