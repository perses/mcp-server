package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	apiClient "github.com/perses/perses/pkg/client/api/v1"
	v1 "github.com/perses/perses/pkg/model/api/v1"
	"github.com/perses/perses/pkg/model/api/v1/variable"
)

func ListGlobalVariables(client apiClient.ClientInterface) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("perses_list_global_variables",
			mcp.WithDescription("List all Global Variables"),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:           "Lists all global variables in Perses",
				ReadOnlyHint:    ToBoolPtr(true),
				DestructiveHint: ToBoolPtr(false),
				IdempotentHint:  ToBoolPtr(true),
				OpenWorldHint:   ToBoolPtr(false),
			})),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {

			variables, err := client.GlobalVariable().List("")
			if err != nil {
				return nil, fmt.Errorf("error retrieving global variables: %w", err)
			}

			variablesJSON, err := json.Marshal(variables)
			if err != nil {
				return nil, fmt.Errorf("error marshalling global variables: %w", err)
			}
			return mcp.NewToolResultText(string(variablesJSON)), nil
		}
}

func GetGlobalVariableByName(client apiClient.ClientInterface) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("perses_get_global_variable_by_name",
			mcp.WithDescription("Get a global variable by name"),
			mcp.WithString("name", mcp.Required(),
				mcp.Description("Global Variable name")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:           "Gets a global variable by name in Perses",
				ReadOnlyHint:    ToBoolPtr(true),
				DestructiveHint: ToBoolPtr(false),
				IdempotentHint:  ToBoolPtr(true),
				OpenWorldHint:   ToBoolPtr(false),
			})),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			name, err := request.RequireString("name")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			globalVariable, err := client.GlobalVariable().Get(name)
			if err != nil {
				return nil, fmt.Errorf("error retrieving global variable '%s': %w", name, err)
			}

			globalVariableJSON, err := json.Marshal(globalVariable)
			if err != nil {
				return nil, fmt.Errorf("error marshalling global variable '%s': %w", name, err)
			}
			return mcp.NewToolResultText(string(globalVariableJSON)), nil
		}
}

func ListProjectVariables(client apiClient.ClientInterface) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("perses_list_project_variables",
			mcp.WithDescription("List variables for a specific project"),
			mcp.WithString("project", mcp.Required(),
				mcp.Description("Project name")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:           "Lists variables for a specific project in Perses",
				ReadOnlyHint:    ToBoolPtr(true),
				DestructiveHint: ToBoolPtr(false),
				IdempotentHint:  ToBoolPtr(true),
				OpenWorldHint:   ToBoolPtr(false),
			})),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			project, err := request.RequireString("project")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			variables, err := client.Variable(project).List("")
			if err != nil {
				return nil, fmt.Errorf("error retrieving variables in project '%s': %w", project, err)
			}

			variablesJSON, err := json.Marshal(variables)
			if err != nil {
				return nil, fmt.Errorf("error marshalling variables: %w", err)
			}
			return mcp.NewToolResultText(string(variablesJSON)), nil
		}
}

func GetProjectVariableByName(client apiClient.ClientInterface) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("perses_get_project_variable_by_name",
			mcp.WithDescription("Get a variable by name in a specific project"),
			mcp.WithString("project", mcp.Required(),
				mcp.Description("Project name")),
			mcp.WithString("name", mcp.Required(),
				mcp.Description("Variable name")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:           "Gets a variable by name in a specific project in Perses",
				ReadOnlyHint:    ToBoolPtr(true),
				DestructiveHint: ToBoolPtr(false),
				IdempotentHint:  ToBoolPtr(true),
				OpenWorldHint:   ToBoolPtr(false),
			})),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			project, err := request.RequireString("project")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			name, err := request.RequireString("name")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			variable, err := client.Variable(project).Get(name)
			if err != nil {
				return nil, fmt.Errorf("error retrieving variable '%s' in project '%s': %w", name, project, err)
			}

			variableJSON, err := json.Marshal(variable)
			if err != nil {
				return nil, fmt.Errorf("error marshalling variable: %w", err)
			}
			return mcp.NewToolResultText(string(variableJSON)), nil
		}
}

func CreateProjectTextVariable(client apiClient.ClientInterface) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("perses_create_project_variable",
			mcp.WithDescription("Create a project level variable"),
			mcp.WithString("name", mcp.Required(),
				mcp.Description("Variable name")),
			mcp.WithString("project", mcp.Required(),
				mcp.Description("Project name")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:           "Creates a project level variable in Perses",
				ReadOnlyHint:    ToBoolPtr(false),
				DestructiveHint: ToBoolPtr(false),
				IdempotentHint:  ToBoolPtr(true),
				OpenWorldHint:   ToBoolPtr(false),
			})),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			name, err := request.RequireString("name")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			project, err := request.RequireString("project")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			projectVar := &v1.Variable{
				Kind: "Variable",
				Metadata: v1.ProjectMetadata{
					Metadata: v1.Metadata{
						Name: name,
					},
					ProjectMetadataWrapper: v1.ProjectMetadataWrapper{
						Project: project,
					},
				},
				Spec: v1.VariableSpec{
					Kind: variable.KindText,
					Spec: &variable.TextSpec{
						Value: name,
					},
				},
			}

			result, err := client.Variable(project).Create(projectVar)
			if err != nil {
				return nil, fmt.Errorf("error creating variable '%s' in project '%s': %w", name, project, err)
			}

			resultJSON, err := json.Marshal(result)
			if err != nil {
				return nil, fmt.Errorf("error marshalling variable result: %w", err)
			}

			return mcp.NewToolResultText(string(resultJSON)), nil
		}
}
