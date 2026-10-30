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
	"errors"
	"fmt"
	"net/http"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/perses/mcp-server/pkg/tools"
	"github.com/perses/mcp-server/pkg/tools/resource"
	apiclient "github.com/perses/perses/pkg/client/api"
	"github.com/perses/perses/pkg/client/perseshttp"
)

type validation struct {
	client apiclient.ClientInterface
}

func New(client apiclient.ClientInterface) resource.Resource {
	return &validation{client: client}
}

func (v *validation) GetTools() []*tools.Tool {
	client := v.client.Validate()
	return []*tools.Tool{
		tool("perses_validate_dashboard", "dashboard", "a dashboard", client.Dashboard),
		tool("perses_validate_datasource", "datasource", "a project datasource", client.Datasource),
		tool("perses_validate_global_datasource", "datasource", "a global datasource", client.GlobalDatasource),
		tool("perses_validate_variable", "variable", "a project variable", client.Variable),
		tool("perses_validate_global_variable", "variable", "a global variable", client.GlobalVariable),
	}
}

func tool[T any](name, inputField, subject string, validate func(*T) error) *tools.Tool {
	definition := &mcp.Tool{
		Name: name,
		Description: "Validate " + subject + " JSON against the installed plugin CUE schemas. " +
			"This does not create or modify anything. Returns `{\"valid\": true}` or " +
			"`{\"valid\": false, \"errors\": \"...\"}`.",
		InputSchema: &jsonschema.Schema{
			Type: tools.SchemaTypeObject,
			Properties: map[string]*jsonschema.Schema{
				inputField: {
					Type:        tools.SchemaTypeString,
					Description: "JSON for " + subject + " as a string",
				},
			},
			Required: []string{inputField},
		},
		Annotations: &mcp.ToolAnnotations{
			Title:           "Validates " + subject + " against Perses schemas",
			ReadOnlyHint:    true,
			DestructiveHint: new(false),
			IdempotentHint:  true,
			OpenWorldHint:   new(false),
		},
	}

	handler := func(_ context.Context, _ *mcp.CallToolRequest, input map[string]any) (*mcp.CallToolResult, any, error) { //nolint:unparam
		payload, ok := input[inputField].(string)
		if !ok {
			return nil, nil, fmt.Errorf("input field %q must be a JSON string", inputField)
		}
		var entity T
		var result validationResult
		if err := json.Unmarshal([]byte(payload), &entity); err != nil {
			result.Errors = err.Error()
		} else if err := validate(&entity); err != nil {
			var requestError *perseshttp.RequestError
			if !errors.As(err, &requestError) || requestError.StatusCode != http.StatusBadRequest {
				return nil, nil, fmt.Errorf("error validating %s: %w", subject, err)
			}
			result.Errors = requestError.Message
		} else {
			result.Valid = true
		}

		resultJSON, err := json.Marshal(result)
		if err != nil {
			return nil, nil, fmt.Errorf("error marshalling validation result: %w", err)
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: string(resultJSON)}},
		}, nil, nil
	}

	return &tools.Tool{
		MCPTool:      definition,
		IsWriteTool:  false,
		ResourceType: tools.ValidateResource,
		RegisterWith: func(server *mcp.Server) { mcp.AddTool(server, definition, handler) },
	}
}

type validationResult struct {
	Valid  bool   `json:"valid"`
	Errors string `json:"errors,omitempty"`
}

func (v *validation) Create() *tools.Tool { return nil }
func (v *validation) Update() *tools.Tool { return nil }
func (v *validation) Delete() *tools.Tool { return nil }
func (v *validation) List() *tools.Tool   { return nil }
func (v *validation) Get() *tools.Tool    { return nil }
