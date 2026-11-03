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

package variable

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/perses/mcp-server/pkg/tools"
	"github.com/perses/mcp-server/pkg/tools/resource"
	apiClient "github.com/perses/perses/pkg/client/api/v1"
	v1 "github.com/perses/perses/pkg/model/api/v1"
	"github.com/perses/spec/go/dashboard/variable"
)

type projectVariable struct {
	client apiClient.ClientInterface
}

func New(client apiClient.ClientInterface) resource.Resource {
	return &projectVariable{
		client: client,
	}
}

func (v *projectVariable) GetTools() []*tools.Tool {
	return []*tools.Tool{
		v.List(),
		v.Get(),
		v.Create(),
		v.Update(),
		v.Delete(),
	}
}

type ListProjectVariablesInput struct {
	Project string `json:"project" jsonschema:"Project name"`
}

func (v *projectVariable) List() *tools.Tool {
	tool := &mcp.Tool{
		Name:        "perses_list_project_variables",
		Description: "List the variables shared by all dashboards of one project (not the variables defined inside a dashboard; those appear in perses_get_dashboard_by_name). Global variables are listed by perses_list_global_variables.",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Lists variables for a specific project in Perses",
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
					Description: "Project name (metadata.name from perses_list_projects)",
					MinLength:   new(1),
					MaxLength:   new(75),
					Pattern:     tools.PatternResourceName,
				},
			},
			Required: []string{string(tools.ProjectResource)},
		},
	}

	handler := func(_ context.Context, _ *mcp.CallToolRequest, input ListProjectVariablesInput) (*mcp.CallToolResult, any, error) { //nolint:unparam
		variables, err := v.client.Variable(input.Project).List("")
		if err != nil {
			return nil, nil, fmt.Errorf("error retrieving variables in project '%s': %w", input.Project, err)
		}

		variablesJSON, err := json.Marshal(variables)
		if err != nil {
			return nil, nil, fmt.Errorf("error marshalling variables: %w", err)
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: string(variablesJSON),
				},
			},
		}, nil, nil
	}

	return &tools.Tool{
		MCPTool:      tool,
		IsWriteTool:  false,
		ResourceType: tools.VariableResource,
		RegisterWith: func(server *mcp.Server) { mcp.AddTool(server, tool, handler) },
	}
}

type GetProjectVariableByNameInput struct {
	Project string `json:"project" jsonschema:"Project name"`
	Name    string `json:"name" jsonschema:"Variable name"`
}

func (v *projectVariable) Get() *tools.Tool {
	tool := &mcp.Tool{
		Name:        "perses_get_project_variable_by_name",
		Description: "Get one project-level variable by name: its kind (TextVariable or ListVariable), value or list plugin settings, and metadata.version. Variables defined inside a dashboard are not project variables; read those with perses_get_dashboard_by_name. Fails if the name does not exist in the project; see perses_list_project_variables.",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Gets a variable by name in a specific project in Perses",
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
					Description: "Project name (metadata.name from perses_list_projects)",
					MinLength:   new(1),
					MaxLength:   new(75),
					Pattern:     tools.PatternResourceName,
				},
				tools.ResourceName: {
					Type:        tools.SchemaTypeString,
					Description: "Variable name",
					MinLength:   new(1),
					MaxLength:   new(75),
					Pattern:     tools.PatternResourceName,
				},
			},
			Required: []string{string(tools.ProjectResource), tools.ResourceName},
		},
	}

	handler := func(_ context.Context, _ *mcp.CallToolRequest, input GetProjectVariableByNameInput) (*mcp.CallToolResult, any, error) { //nolint:unparam
		projectVar, err := v.client.Variable(input.Project).Get(input.Name)
		if err != nil {
			return nil, nil, fmt.Errorf("error retrieving variable '%s' in project '%s': %w", input.Name, input.Project, err)
		}

		variableJSON, err := json.Marshal(projectVar)
		if err != nil {
			return nil, nil, fmt.Errorf("error marshalling variable: %w", err)
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: string(variableJSON),
				},
			},
		}, nil, nil
	}

	return &tools.Tool{
		MCPTool:      tool,
		IsWriteTool:  false,
		ResourceType: tools.VariableResource,
		RegisterWith: func(server *mcp.Server) { mcp.AddTool(server, tool, handler) },
	}
}

type CreateProjectVariableInput struct {
	Name    string `json:"name" jsonschema:"Variable name"`
	Project string `json:"project" jsonschema:"Project name"`
	Value   string `json:"value" jsonschema:"Variable value (for TextVariable)"`
}

func (v *projectVariable) Create() *tools.Tool {
	tool := &mcp.Tool{
		Name:        "perses_create_project_variable",
		Description: "Create a project-level TextVariable (a constant string, referenced in queries as $name) shared by all dashboards of the project. To add a variable to a single dashboard, or a ListVariable, use perses_set_dashboard_variables instead. Fails if the name exists.",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Creates a project level variable in Perses",
			ReadOnlyHint:    false,
			DestructiveHint: new(false),
			IdempotentHint:  true,
			OpenWorldHint:   new(false),
		},
		InputSchema: &jsonschema.Schema{
			Type: tools.SchemaTypeObject,
			Properties: map[string]*jsonschema.Schema{
				tools.ResourceName: {
					Type:        tools.SchemaTypeString,
					Description: "Variable name",
					MinLength:   new(1),
					MaxLength:   new(75),
					Pattern:     tools.PatternResourceName,
				},
				string(tools.ProjectResource): {
					Type:        tools.SchemaTypeString,
					Description: "Project name (metadata.name from perses_list_projects)",
					MinLength:   new(1),
					MaxLength:   new(75),
					Pattern:     tools.PatternResourceName,
				},
				"value": {
					Type:        tools.SchemaTypeString,
					Description: "Variable value. Empty when not provided.",
				},
			},
			Required: []string{tools.ResourceName, string(tools.ProjectResource)},
		},
	}

	handler := func(_ context.Context, _ *mcp.CallToolRequest, input CreateProjectVariableInput) (*mcp.CallToolResult, any, error) { //nolint:unparam
		projectVar := &v1.Variable{
			Kind: "Variable",
			Metadata: v1.ProjectMetadata{
				Metadata: v1.Metadata{
					Name: input.Name,
				},
				ProjectMetadataWrapper: v1.ProjectMetadataWrapper{
					Project: input.Project,
				},
			},
			Spec: v1.VariableSpec{
				Kind: variable.KindText,
				Spec: &variable.TextSpec{
					Value: input.Value,
				},
			},
		}

		result, err := v.client.Variable(input.Project).Create(projectVar)
		if err != nil {
			return nil, nil, fmt.Errorf("error creating variable '%s' in project '%s': %w", input.Name, input.Project, err)
		}

		resultJSON, err := json.Marshal(result)
		if err != nil {
			return nil, nil, fmt.Errorf("error marshalling variable result: %w", err)
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: string(resultJSON),
				},
			},
		}, nil, nil
	}

	return &tools.Tool{
		MCPTool:      tool,
		IsWriteTool:  true,
		ResourceType: tools.VariableResource,
		RegisterWith: func(server *mcp.Server) { mcp.AddTool(server, tool, handler) },
	}
}

type UpdateProjectVariableInput struct {
	Name    string `json:"name" jsonschema:"Variable name"`
	Project string `json:"project" jsonschema:"Project name"`
	Value   string `json:"value" jsonschema:"Variable value (for TextVariable)"`
}

func (v *projectVariable) Update() *tools.Tool {
	tool := &mcp.Tool{
		Name:        "perses_update_project_variable",
		Description: "Set the value of an existing project-level variable; it is rewritten as a TextVariable (any list configuration is replaced).",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Updates a project level variable in Perses",
			ReadOnlyHint:    false,
			DestructiveHint: new(false),
			IdempotentHint:  true,
			OpenWorldHint:   new(false),
		},
		InputSchema: &jsonschema.Schema{
			Type: tools.SchemaTypeObject,
			Properties: map[string]*jsonschema.Schema{
				tools.ResourceName: {
					Type:        tools.SchemaTypeString,
					Description: "Variable name",
					MinLength:   new(1),
					MaxLength:   new(75),
					Pattern:     tools.PatternResourceName,
				},
				string(tools.ProjectResource): {
					Type:        tools.SchemaTypeString,
					Description: "Project name (metadata.name from perses_list_projects)",
					MinLength:   new(1),
					MaxLength:   new(75),
					Pattern:     tools.PatternResourceName,
				},
				"value": {
					Type:        tools.SchemaTypeString,
					Description: "Variable value",
				},
			},
			Required: []string{tools.ResourceName, string(tools.ProjectResource), "value"},
		},
	}

	handler := func(_ context.Context, _ *mcp.CallToolRequest, input UpdateProjectVariableInput) (*mcp.CallToolResult, any, error) { //nolint:unparam
		projectVar := &v1.Variable{
			Kind: v1.KindVariable,
			Metadata: v1.ProjectMetadata{
				Metadata: v1.Metadata{
					Name: input.Name,
				},
				ProjectMetadataWrapper: v1.ProjectMetadataWrapper{
					Project: input.Project,
				},
			},
			Spec: v1.VariableSpec{
				Kind: variable.KindText,
				Spec: &variable.TextSpec{
					Value: input.Value,
				},
			},
		}

		result, err := v.client.Variable(input.Project).Update(projectVar)
		if err != nil {
			return nil, nil, fmt.Errorf("error updating variable '%s' in project '%s': %w", input.Name, input.Project, err)
		}

		resultJSON, err := json.Marshal(result)
		if err != nil {
			return nil, nil, fmt.Errorf("error marshalling updated variable: %w", err)
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: string(resultJSON),
				},
			},
		}, nil, nil
	}

	return &tools.Tool{
		MCPTool:      tool,
		IsWriteTool:  true,
		ResourceType: tools.VariableResource,
		RegisterWith: func(server *mcp.Server) { mcp.AddTool(server, tool, handler) },
	}
}

type DeleteProjectVariableInput struct {
	Name    string `json:"name" jsonschema:"Variable name to delete"`
	Project string `json:"project" jsonschema:"Project name"`
}

func (v *projectVariable) Delete() *tools.Tool {
	tool := &mcp.Tool{
		Name:        "perses_delete_project_variable",
		Description: "Permanently delete a project-level variable. Dashboards of the project that reference $name in queries will no longer resolve it unless a global variable with the same name exists. Cannot be undone; to only change its value use perses_update_project_variable.",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Deletes a project level variable in Perses",
			ReadOnlyHint:    false,
			DestructiveHint: new(true),
			IdempotentHint:  true,
			OpenWorldHint:   new(false),
		},
		InputSchema: &jsonschema.Schema{
			Type: tools.SchemaTypeObject,
			Properties: map[string]*jsonschema.Schema{
				tools.ResourceName: {
					Type:        tools.SchemaTypeString,
					Description: "Variable name",
					MinLength:   new(1),
					MaxLength:   new(75),
					Pattern:     tools.PatternResourceName,
				},
				string(tools.ProjectResource): {
					Type:        tools.SchemaTypeString,
					Description: "Project name (metadata.name from perses_list_projects)",
					MinLength:   new(1),
					MaxLength:   new(75),
					Pattern:     tools.PatternResourceName,
				},
			},
			Required: []string{tools.ResourceName, string(tools.ProjectResource)},
		},
	}

	handler := func(_ context.Context, _ *mcp.CallToolRequest, input DeleteProjectVariableInput) (*mcp.CallToolResult, any, error) { //nolint:unparam
		err := v.client.Variable(input.Project).Delete(input.Name)
		if err != nil {
			return nil, nil, fmt.Errorf("error deleting variable '%s' in project '%s': %w", input.Name, input.Project, err)
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: fmt.Sprintf("Variable '%s' deleted successfully from project '%s'", input.Name, input.Project),
				},
			},
		}, nil, nil
	}

	return &tools.Tool{
		MCPTool:      tool,
		IsWriteTool:  true,
		ResourceType: tools.VariableResource,
		RegisterWith: func(server *mcp.Server) { mcp.AddTool(server, tool, handler) },
	}
}
