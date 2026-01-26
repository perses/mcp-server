package project

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
	"github.com/perses/perses/pkg/model/api/v1/common"
)

type project struct {
	client apiClient.ClientInterface
}

func NewProject(client apiClient.ClientInterface) resource.Resource {
	return &project{
		client: client,
	}
}

type CreateProjectInput struct {
	Project     string `json:"project" jsonschema:"Name of the project to create"`
	DisplayName string `json:"displayName" jsonschema:"Display name for the project"`
	Description string `json:"description" jsonschema:"Description for the project"`
}

func (p *project) Create() *tools.Tool {
	tool := &mcp.Tool{
		Annotations: &mcp.ToolAnnotations{
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
	handler := func(ctx context.Context, _ *mcp.CallToolRequest, input CreateProjectInput) (*mcp.CallToolResult, any, error) {
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
		response, err := p.client.Project().Create(newProjectRequest)
		if err != nil {
			return nil, nil, fmt.Errorf("error creating project '%s': %w", input.Project, err)
		}
		text, err := json.Marshal(response)
		if err != nil {
			return nil, nil, fmt.Errorf("error marshalling created project '%s': %w", input.Project, err)
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: string(text),
				},
			},
		}, nil, nil
	}
	return &tools.Tool{
		MCPTool:      tool,
		IsWriteTool:  true,
		ResourceType: tools.ProjectResource,
		RegisterWith: func(server *mcp.Server) {
			mcp.AddTool(server, tool, handler)
		},
	}
}

type ListProjectsInput struct{}

func (p *project) List() *tools.Tool {
	tool := &mcp.Tool{
		Name:        "perses_list_projects",
		Description: "List all Perses projects",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Lists all projects in Perses",
			ReadOnlyHint:    true,
			DestructiveHint: jsonschema.Ptr(false),
			IdempotentHint:  true,
			OpenWorldHint:   jsonschema.Ptr(false),
		},
	}
	handler := func(ctx context.Context, _ *mcp.CallToolRequest, input ListProjectsInput) (*mcp.CallToolResult, any, error) {
		projects, err := p.client.Project().List("")
		if err != nil {
			return nil, nil, fmt.Errorf("error retrieving projects: %w", err)
		}
		text, err := json.Marshal(projects)
		if err != nil {
			return nil, nil, fmt.Errorf("error marshalling projects: %w", err)
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: string(text),
				},
			},
		}, nil, nil
	}
	return &tools.Tool{
		MCPTool:      tool,
		IsWriteTool:  false,
		ResourceType: tools.ProjectResource,
		RegisterWith: func(server *mcp.Server) {
			mcp.AddTool(server, tool, handler)
		},
	}
}

type UpdateProjectInput struct {
	Name        string `json:"name" jsonschema:"Name of the project to update"`
	DisplayName string `json:"displayName" jsonschema:"Display name for the project"`
	Description string `json:"description" jsonschema:"Description for the project"`
}

func (p *project) Update() *tools.Tool {
	tool := &mcp.Tool{
		Name:        "perses_update_project",
		Description: "Update an existing Perses Project",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Updates an existing project in Perses",
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
					Description: "Name of the project to update",
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
				},
			},
			Required: []string{"name"},
		},
	}
	handler := func(ctx context.Context, _ *mcp.CallToolRequest, input UpdateProjectInput) (*mcp.CallToolResult, any, error) {
		updateProjectRequest := &v1.Project{
			Kind: "Project",
			Metadata: v1.Metadata{
				Name: input.Name,
			},
			Spec: v1.ProjectSpec{
				Display: &common.Display{
					Name:        input.DisplayName,
					Description: input.Description,
				},
			},
		}
		response, err := p.client.Project().Update(updateProjectRequest)
		if err != nil {
			return nil, nil, fmt.Errorf("error updating project '%s': %w", input.Name, err)
		}
		text, err := json.Marshal(response)
		if err != nil {
			return nil, nil, fmt.Errorf("error marshalling updated project '%s': %w", input.Name, err)
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: string(text),
				},
			},
		}, nil, nil
	}
	return &tools.Tool{
		MCPTool:      tool,
		IsWriteTool:  true,
		ResourceType: tools.ProjectResource,
		RegisterWith: func(server *mcp.Server) {
			mcp.AddTool(server, tool, handler)
		},
	}
}

type DeleteProjectInput struct {
	Name string `json:"name" jsonschema:"Name of the project to delete"`
}

func (p *project) Delete() *tools.Tool {
	tool := &mcp.Tool{
		Name:        "perses_delete_project",
		Description: "Delete a Perses project",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Deletes a project in Perses",
			ReadOnlyHint:    false,
			DestructiveHint: jsonschema.Ptr(true),
			IdempotentHint:  true,
			OpenWorldHint:   jsonschema.Ptr(false),
		},
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"name": {
					Type:        "string",
					Description: "Name of the project to delete",
					MinLength:   jsonschema.Ptr(1),
					MaxLength:   jsonschema.Ptr(75),
					Pattern:     "^[a-zA-Z0-9_.-]+$",
				},
			},
			Required: []string{"name"},
		},
	}
	handler := func(ctx context.Context, _ *mcp.CallToolRequest, input DeleteProjectInput) (*mcp.CallToolResult, any, error) {
		err := p.client.Project().Delete(input.Name)
		if err != nil {
			return nil, nil, fmt.Errorf("error deleting project '%s': %w", input.Name, err)
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: fmt.Sprintf("Project '%s' deleted successfully", input.Name),
				},
			},
		}, nil, nil
	}
	return &tools.Tool{
		MCPTool:      tool,
		IsWriteTool:  true,
		ResourceType: tools.ProjectResource,
		RegisterWith: func(server *mcp.Server) {
			mcp.AddTool(server, tool, handler)
		},
	}
}

type GetProjectInput struct {
	Name string `json:"name" jsonschema:"Name of the project to retrieve"`
}

func (p *project) Get() *tools.Tool {
	tool := &mcp.Tool{
		Name:        "perses_get_project_by_name",
		Description: "Get a project by name in Perses",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Gets a project by name in Perses",
			ReadOnlyHint:    true,
			DestructiveHint: jsonschema.Ptr(false),
			IdempotentHint:  true,
			OpenWorldHint:   jsonschema.Ptr(false),
		},
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"name": {
					Type:        "string",
					Description: "Name of the project to retrieve",
					MinLength:   jsonschema.Ptr(1),
					MaxLength:   jsonschema.Ptr(75),
					Pattern:     "^[a-zA-Z0-9_.-]+$",
				},
			},
			Required: []string{"name"},
		},
	}
	handler := func(ctx context.Context, _ *mcp.CallToolRequest, input GetProjectInput) (*mcp.CallToolResult, any, error) {
		response, err := p.client.Project().Get(input.Name)
		if err != nil {
			return nil, nil, fmt.Errorf("error retrieving project '%s': %w", input.Name, err)
		}
		text, err := json.Marshal(response)
		if err != nil {
			return nil, nil, fmt.Errorf("error marshalling project '%s': %w", input.Name, err)
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: string(text),
				},
			},
		}, nil, nil
	}
	return &tools.Tool{
		MCPTool:      tool,
		IsWriteTool:  false,
		ResourceType: tools.ProjectResource,
		RegisterWith: func(server *mcp.Server) {
			mcp.AddTool(server, tool, handler)
		},
	}
}

func (p *project) GetTools() []*tools.Tool {
	return []*tools.Tool{
		p.List(),
		p.Get(),
		p.Create(),
		p.Update(),
		p.Delete(),
	}
}
