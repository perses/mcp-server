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
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/perses/mcp-server/pkg/tools"
	v1 "github.com/perses/perses/pkg/model/api/v1"
)

type ValidateVariableInput struct {
	Variable json.RawMessage `json:"variable"`
}

func (v *validation) Variable() *tools.Tool {
	tool := &mcp.Tool{
		Name: "perses_validate_variable",
		Description: "Validate a variable against the plugin schemas without saving it, its kind (Variable or " +
			"GlobalVariable) telling the scope. Returns {valid: true}, or {valid: false, error} when the kind is " +
			"missing or not a variable kind, or when Perses rejects the variable. A tool error means the variable " +
			"could not be validated (e.g. Perses unreachable or a server error).",
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
					Description: "Variable to validate, as a JSON object with kind Variable (project) or GlobalVariable.",
				},
			},
			Required: []string{"variable"},
		},
	}

	handler := func(_ context.Context, _ *mcp.CallToolRequest, input ValidateVariableInput) (*mcp.CallToolResult, any, error) { //nolint:unparam
		result, err := v.validateVariable(input)
		if err != nil {
			return nil, nil, err
		}
		toolResult, err := tools.JSONResult(result)
		if err != nil {
			return nil, nil, fmt.Errorf("error marshalling variable validation: %w", err)
		}
		return toolResult, nil, nil
	}

	return &tools.Tool{
		MCPTool:      tool,
		IsWriteTool:  false,
		ResourceType: tools.ValidateResource,
		RegisterWith: func(server *mcp.Server) { mcp.AddTool(server, tool, handler) },
	}
}

func (v *validation) validateVariable(input ValidateVariableInput) (*result, error) {
	switch kind := kindOf(input.Variable); {
	case strings.EqualFold(kind, string(v1.KindGlobalVariable)):
		entity := &v1.GlobalVariable{}
		if err := json.Unmarshal(input.Variable, entity); err != nil {
			return invalid(err), nil
		}
		return serverResult(v.client.Validate().GlobalVariable(entity))
	case strings.EqualFold(kind, string(v1.KindVariable)):
		entity := &v1.Variable{}
		if err := json.Unmarshal(input.Variable, entity); err != nil {
			return invalid(err), nil
		}
		return serverResult(v.client.Validate().Variable(entity))
	default:
		return invalid(fmt.Errorf("kind must be %q or %q, got %q", v1.KindVariable, v1.KindGlobalVariable, kind)), nil
	}
}
