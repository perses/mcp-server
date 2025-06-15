package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	apiClient "github.com/perses/perses/pkg/client/api/v1"
	v1 "github.com/perses/perses/pkg/model/api/v1"
	"github.com/perses/perses/pkg/model/api/v1/common"
)

func ListProjects(client apiClient.ClientInterface) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("perses_list_projects",
			mcp.WithDescription("List all Perses Projects")),
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
			mcp.WithString("name", mcp.Required(),
				mcp.Description("Project name"))),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			name, err := request.RequireString("name")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			project, err := client.Project().Get(name)
			if err != nil {
				return nil, fmt.Errorf("error retrieving project '%s': %w", name, err)
			}

			projectJSON, err := json.Marshal(project)
			if err != nil {
				return nil, fmt.Errorf("error marshalling project '%s': %w", name, err)
			}
			return mcp.NewToolResultText(string(projectJSON)), nil
		}
}

func CreateProject(client apiClient.ClientInterface) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("perses_create_project",
			mcp.WithDescription("Create a new Perses Project"), mcp.WithString("project", mcp.Required())),
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
