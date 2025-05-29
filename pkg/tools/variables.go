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
			mcp.WithDescription("List all Global Variables")),
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

func ListVariables(client apiClient.ClientInterface) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("perses_list_variables",
			mcp.WithDescription("List variables for a specific project"),
			mcp.WithString("project", mcp.Required(),
				mcp.Description("Project name"))),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			project, ok := request.Params.Arguments["project"].(string)
			if !ok {
				return mcp.NewToolResultError("invalid type for 'project', expected string"), nil
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

func CreateProjectTextVariable(client apiClient.ClientInterface) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("perses_create_project_variable",
			mcp.WithDescription("Create a project level variable"),
			mcp.WithString("name", mcp.Required(),
				mcp.Description("Variable name")),
			mcp.WithString("project", mcp.Required(),
				mcp.Description("Project name"))),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			name, ok := request.Params.Arguments["name"].(string)
			if !ok {
				return mcp.NewToolResultError("invalid type for 'name', expected string"), nil
			}

			project, ok := request.Params.Arguments["project"].(string)
			if !ok {
				return mcp.NewToolResultError("invalid type for 'project', expected string"), nil
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
