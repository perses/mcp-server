package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	newMcp "github.com/modelcontextprotocol/go-sdk/mcp"
	apiClient "github.com/perses/perses/pkg/client/api/v1"
	v1 "github.com/perses/perses/pkg/model/api/v1"
	"github.com/perses/perses/pkg/model/api/v1/common"
)

func ListNewProjects(client apiClient.ClientInterface) (*newMcp.Tool, newMcp.ToolHandlerFor[map[string]any, any]) {
	tool := &newMcp.Tool{
		Name:        "perses_list_projects",
		Description: "List all Perses projects",
		Annotations: &newMcp.ToolAnnotations{
			ReadOnlyHint:   true,
			IdempotentHint: true,
		},
	}

	handler := func(ctx context.Context, _ *newMcp.CallToolRequest, input map[string]any) (result *newMcp.CallToolResult, output any, _ error) {
		projects, err := client.Project().List("")
		if err != nil {
			return nil, nil, fmt.Errorf("error retrieving projects: %w", err)
		}

		projectsJSON, err := json.Marshal(projects)
		if err != nil {
			return nil, nil, fmt.Errorf("error marshalling projects: %w", err)
		}

		return nil, string(projectsJSON), nil
	}

	return tool, handler
}

func ListProjects(client apiClient.ClientInterface) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("perses_list_projects",
			mcp.WithDescription("List all Perses Projects"),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:           "Lists all projects in Perses",
				ReadOnlyHint:    ToBoolPtr(true),
				DestructiveHint: ToBoolPtr(false),
				IdempotentHint:  ToBoolPtr(true),
				OpenWorldHint:   ToBoolPtr(false),
			})),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {

			projects, err := client.Project().List("")
			if err != nil {
				return nil, fmt.Errorf("error retrieving projects: %w", err)
			}

			projectsJSON, err := json.Marshal(projects)
			if err != nil {
				return nil, fmt.Errorf("error marshalling projects: %w", err)
			}
			return mcp.NewToolResultText(string(projectsJSON)), nil
		}
}

func GetProjectByName(client apiClient.ClientInterface) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("perses_get_project_by_name",
			mcp.WithDescription("Get a project by name"),
			mcp.WithString("project", mcp.Required(),
				mcp.Description("Project name")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:           "Gets a project by name in Perses",
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

			response, err := client.Project().Get(project)
			if err != nil {
				return nil, fmt.Errorf("error retrieving project '%s': %w", project, err)
			}

			projectJSON, err := json.Marshal(response)
			if err != nil {
				return nil, fmt.Errorf("error marshalling project '%s': %w", project, err)
			}
			return mcp.NewToolResultText(string(projectJSON)), nil
		}
}

func CreateProject(client apiClient.ClientInterface) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("perses_create_project",
			mcp.WithDescription("Create a new Perses Project"),
			mcp.WithString("project",
				mcp.Required(),
				mcp.Description("Project name")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:           "Creates a new project in Perses",
				ReadOnlyHint:    ToBoolPtr(false),
				DestructiveHint: ToBoolPtr(false),
				IdempotentHint:  ToBoolPtr(true),
				OpenWorldHint:   ToBoolPtr(false),
			})),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			project, err := request.RequireString("project")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			newProjectRequest := &v1.Project{
				Kind: "Project",
				Metadata: v1.Metadata{
					Name: project,
				},
				Spec: v1.ProjectSpec{
					Display: &common.Display{
						Name: project,
					},
				},
			}

			response, err := client.Project().Create(newProjectRequest)
			if err != nil {
				return nil, fmt.Errorf("error creating project '%s': %w", project, err)
			}
			projectJSON, err := json.Marshal(response)
			if err != nil {
				return nil, fmt.Errorf("error marshalling project '%s': %w", project, err)
			}
			return mcp.NewToolResultText(string(projectJSON)), nil
		}
}
