package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/jsonschema-go/jsonschema"
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

func GetProjectByName(client apiClient.ClientInterface) (*newMcp.Tool, newMcp.ToolHandlerFor[GetProjectByNameInput, *v1.Project]) {
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
			Title:           "Gets a project by name in Perses",
			ReadOnlyHint:    true,
			IdempotentHint:  true,
			DestructiveHint: jsonschema.Ptr(false),
		},
	}

	handler := func(ctx context.Context, _ *newMcp.CallToolRequest, input GetProjectByNameInput) (
		*newMcp.CallToolResult, *v1.Project, error) {

		response, err := client.Project().Get(input.Project)
		if err != nil {
			return nil, nil, fmt.Errorf("error retrieving project '%s': %w", input.Project, err)
		}

		return nil, response, nil
	}

	return tool, handler
}

type CreateProjectInput struct {
	Project     string `json:"project" jsonschema:"Name of the project to create"`
	DisplayName string `json:"displayName" jsonschema:"Display name for the project"`
	Description string `json:"description" jsonschema:"Description for the project"`
}

func CreateProject(client apiClient.ClientInterface) (*newMcp.Tool, newMcp.ToolHandlerFor[CreateProjectInput, *v1.Project]) {
	tool := &newMcp.Tool{
		Annotations: &newMcp.ToolAnnotations{
			Title:           "Creates a new project in Perses",
			ReadOnlyHint:    false,
			DestructiveHint: jsonschema.Ptr(false),
			IdempotentHint:  true,
			OpenWorldHint:   jsonschema.Ptr(false),
		},
		Description: "Create a new Perses Project",
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"project": {
					Type:        "string",
					Description: "Name of the project to create",
					MinLength:   jsonschema.Ptr(1),
					MaxLength:   jsonschema.Ptr(75),
					Pattern:     "^[a-zA-Z0-9_.-]+$",
				},
				"displayName": {
					Type:        "string",
					Description: "Display name for the project",
					MinLength:   jsonschema.Ptr(1),
					MaxLength:   jsonschema.Ptr(75),
				},
				"description": {
					Type:        "string",
					Description: "Description for the project",
					MaxLength:   jsonschema.Ptr(200),
					Pattern:     "^[a-zA-Z0-9_.-]+$",
				},
			},
			Required: []string{"project"},
		},
		Name: "perses_create_project",
	}

	handler := func(ctx context.Context, _ *newMcp.CallToolRequest, input CreateProjectInput) (*newMcp.CallToolResult, *v1.Project, error) {
		newProjectRequest := &v1.Project{
			Kind: "Project",
			Metadata: v1.Metadata{
				Name: input.Project,
			},
			Spec: v1.ProjectSpec{
				Display: &common.Display{
					Name:        input.DisplayName,
					Description: input.Description,
				},
			},
		}

		response, err := client.Project().Create(newProjectRequest)
		if err != nil {
			return nil, nil, fmt.Errorf("error creating project '%s': %w", input.Project, err)
		}

		return nil, response, nil
	}
	return tool, handler
}
