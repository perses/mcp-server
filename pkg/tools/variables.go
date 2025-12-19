package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	apiClient "github.com/perses/perses/pkg/client/api/v1"
	v1 "github.com/perses/perses/pkg/model/api/v1"
	"github.com/perses/perses/pkg/model/api/v1/variable"
)

func ListGlobalVariables(client apiClient.ClientInterface) (mcp.Tool, mcp.ToolHandlerFor[map[string]any, any]) {
	tool := mcp.Tool{
		Name:        "perses_list_global_variables",
		Description: "List all Global Variables",
		Annotations: &mcp.ToolAnnotations{
			Title:          "Lists all global variables in Perses",
			ReadOnlyHint:   true,
			IdempotentHint: true,
			OpenWorldHint:  jsonschema.Ptr(false),
		},
	}

	handler := func(ctx context.Context, _ *mcp.CallToolRequest, input map[string]any) (*mcp.CallToolResult, any, error) {
		variables, err := client.GlobalVariable().List("")
		if err != nil {
			return nil, nil, fmt.Errorf("error retrieving global variables: %w", err)
		}

		variablesJSON, err := json.Marshal(variables)
		if err != nil {
			return nil, nil, fmt.Errorf("error marshalling global variables: %w", err)
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: string(variablesJSON),
				},
			},
		}, nil, nil
	}
	return tool, handler
}

type GetGlobalVariableByNameInput struct {
	Name string `json:"name" jsonschema:"Global Variable name"`
}

func GetGlobalVariableByName(client apiClient.ClientInterface) (mcp.Tool, mcp.ToolHandlerFor[GetGlobalVariableByNameInput, any]) {
	tool := mcp.Tool{
		Name:        "perses_get_global_variable_by_name",
		Description: "Get a global variable by name",
		Annotations: &mcp.ToolAnnotations{
			Title:          "Gets a global variable by name in Perses",
			ReadOnlyHint:   true,
			IdempotentHint: true,
			OpenWorldHint:  jsonschema.Ptr(false),
		},
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"name": {
					Type:        "string",
					Description: "Global Variable name",
					MinLength:   jsonschema.Ptr(1),
					MaxLength:   jsonschema.Ptr(75),
					Pattern:     "^[a-zA-Z0-9_.-]+$",
				},
			},
			Required: []string{"name"},
		},
	}

	handler := func(ctx context.Context, _ *mcp.CallToolRequest, input GetGlobalVariableByNameInput) (*mcp.CallToolResult, any, error) {
		globalVariable, err := client.GlobalVariable().Get(input.Name)
		if err != nil {
			return nil, nil, fmt.Errorf("error retrieving global variable '%s': %w", input.Name, err)
		}

		globalVariableJSON, err := json.Marshal(globalVariable)
		if err != nil {
			return nil, nil, fmt.Errorf("error marshalling global variable '%s': %w", input.Name, err)
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: string(globalVariableJSON),
				},
			},
		}, nil, nil
	}
	return tool, handler
}

type ListProjectVariablesInput struct {
	Project string `json:"project" jsonschema:"Project name"`
}

func ListProjectVariables(client apiClient.ClientInterface) (mcp.Tool, mcp.ToolHandlerFor[ListProjectVariablesInput, any]) {
	tool := mcp.Tool{
		Name:        "perses_list_project_variables",
		Description: "List variables for a specific project",
		Annotations: &mcp.ToolAnnotations{
			Title:          "Lists variables for a specific project in Perses",
			ReadOnlyHint:   true,
			IdempotentHint: true,
			OpenWorldHint:  jsonschema.Ptr(false),
		},
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"project": {
					Type:        "string",
					Description: "Project name",
					MinLength:   jsonschema.Ptr(1),
					MaxLength:   jsonschema.Ptr(75),
					Pattern:     "^[a-zA-Z0-9_.-]+$",
				},
			},
			Required: []string{"project"},
		},
	}

	handler := func(ctx context.Context, _ *mcp.CallToolRequest, input ListProjectVariablesInput) (*mcp.CallToolResult, any, error) {
		variables, err := client.Variable(input.Project).List("")
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
	return tool, handler
}

type GetProjectVariableByNameInput struct {
	Project string `json:"project" jsonschema:"Project name"`
	Name    string `json:"name" jsonschema:"Variable name"`
}

func GetProjectVariableByName(client apiClient.ClientInterface) (mcp.Tool, mcp.ToolHandlerFor[GetProjectVariableByNameInput, any]) {
	tool := mcp.Tool{
		Name:        "perses_get_project_variable_by_name",
		Description: "Get a variable by name in a specific project",
		Annotations: &mcp.ToolAnnotations{
			Title:          "Gets a variable by name in a specific project in Perses",
			ReadOnlyHint:   true,
			IdempotentHint: true,
			OpenWorldHint:  jsonschema.Ptr(false),
		},
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"project": {
					Type:        "string",
					Description: "Project name",
					MinLength:   jsonschema.Ptr(1),
					MaxLength:   jsonschema.Ptr(75),
					Pattern:     "^[a-zA-Z0-9_.-]+$",
				},
				"name": {
					Type:        "string",
					Description: "Variable name",
					MinLength:   jsonschema.Ptr(1),
					MaxLength:   jsonschema.Ptr(75),
					Pattern:     "^[a-zA-Z0-9_.-]+$",
				},
			},
			Required: []string{"project", "name"},
		},
	}

	handler := func(ctx context.Context, _ *mcp.CallToolRequest, input GetProjectVariableByNameInput) (*mcp.CallToolResult, any, error) {
		variable, err := client.Variable(input.Project).Get(input.Name)
		if err != nil {
			return nil, nil, fmt.Errorf("error retrieving variable '%s' in project '%s': %w", input.Name, input.Project, err)
		}

		variableJSON, err := json.Marshal(variable)
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
	return tool, handler
}

type CreateProjectVariableInput struct {
	Name    string `json:"name" jsonschema:"Variable name"`
	Project string `json:"project" jsonschema:"Project name"`
}

func CreateProjectTextVariable(client apiClient.ClientInterface) (mcp.Tool, mcp.ToolHandlerFor[CreateProjectVariableInput, any]) {
	tool := mcp.Tool{
		Name:        "perses_create_project_variable",
		Description: "Create a project level variable",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Creates a project level variable in Perses",
			ReadOnlyHint:    false,
			DestructiveHint: jsonschema.Ptr(false),
			IdempotentHint:  true,
			OpenWorldHint:   jsonschema.Ptr(false),
		},
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"name": {
					Type:        "string",
					Description: "Variable name",
					MinLength:   jsonschema.Ptr(1),
					MaxLength:   jsonschema.Ptr(75),
					Pattern:     "^[a-zA-Z0-9_.-]+$",
				},
				"project": {
					Type:        "string",
					Description: "Project name",
					MinLength:   jsonschema.Ptr(1),
					MaxLength:   jsonschema.Ptr(75),
					Pattern:     "^[a-zA-Z0-9_.-]+$",
				},
			},
			Required: []string{"name", "project"},
		},
	}

	handler := func(ctx context.Context, _ *mcp.CallToolRequest, input CreateProjectVariableInput) (*mcp.CallToolResult, any, error) {
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
					Value: input.Name,
				},
			},
		}

		result, err := client.Variable(input.Project).Create(projectVar)
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
	return tool, handler
}
