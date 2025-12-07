package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	newMcp "github.com/modelcontextprotocol/go-sdk/mcp"
	apiClient "github.com/perses/perses/pkg/client/api/v1"
	v1 "github.com/perses/perses/pkg/model/api/v1"
	"github.com/perses/perses/pkg/model/api/v1/common"
)

func ListProjects(client apiClient.ClientInterface) (*newMcp.Tool, newMcp.ToolHandlerFor[map[string]any, any]) {
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

type GetProjectByNameInput struct {
	Project string `json:"project" jsonschema:"Project name to retrieve"`
}

type GetProjectByNameOutput struct {
	Project *v1.Project `json:"project" jsonschema:"The project data"`
}

func GetProjectByName(client apiClient.ClientInterface) (*newMcp.Tool, newMcp.ToolHandlerFor[GetProjectByNameInput, GetProjectByNameOutput]) {
	tool := &newMcp.Tool{
		Name:        "perses_get_project_by_name",
		Description: "Get a project by name in Perses",
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"project": {
					Type:        "string",
					Description: "Project name",
					MinLength:   jsonschema.Ptr(1),
				},
			},
		},
		Annotations: &newMcp.ToolAnnotations{
			Title:          "Gets a project by name in Perses",
			ReadOnlyHint:   true,
			IdempotentHint: true,
		},
	}

	handler := func(ctx context.Context, _ *newMcp.CallToolRequest, input GetProjectByNameInput) (
		*newMcp.CallToolResult, GetProjectByNameOutput, error) {

		// Input is already validated and parsed by the SDK
		response, err := client.Project().Get(input.Project)
		if err != nil {
			return nil, GetProjectByNameOutput{}, fmt.Errorf("error retrieving project '%s': %w", input.Project, err)
		}

		// Return structured output - SDK will auto-marshal to JSON
		return nil, GetProjectByNameOutput{Project: response}, nil
	}

	return tool, handler
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
