// Copyright 2025 The Perses Authors
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
package tools
import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	apiClient "github.com/perses/perses/pkg/client/api/v1"
	v1 "github.com/perses/perses/pkg/model/api/v1"
	"github.com/perses/perses/pkg/model/api/v1/common"
)
type ProjectInterface interface {
	List() *Tool
	Get() *Tool
	Create() *Tool
	GetTools() []*Tool
}
type project struct {
	ProjectInterface
	client apiClient.ClientInterface
}
func newProject(client apiClient.ClientInterface) ProjectInterface {
	return &project{
		client: client,
	}
}
func (p *project) GetTools() []*Tool {
	return []*Tool{
		p.List(),
		p.Get(),
		p.Create(),
	}
}
func (p *project) List() *Tool {
	tool := &mcp.Tool{
		Name:        "perses_list_projects",
		Description: "List all Perses projects",
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint:    true,
			IdempotentHint:  true,
			DestructiveHint: jsonschema.Ptr(false),
		},
	}
	handler := func(ctx context.Context, _ *mcp.CallToolRequest, input map[string]any) (*mcp.CallToolResult, any, error) {
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
	return &Tool{
		MCPTool:     tool,
		IsWriteTool: false,
		RegisterWith: func(server *mcp.Server) {
			mcp.AddTool(server, tool, handler)
		},
	
	}
}
type GetProjectByNameInput struct {
	Project string `json:"project" jsonschema:"Project name to retrieve"`
}
func (p *project) Get() *Tool {
	tool := &mcp.Tool{
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
		Annotations: &mcp.ToolAnnotations{
			Title:           "Gets a project by name in Perses",
			ReadOnlyHint:    true,
			IdempotentHint:  true,
			DestructiveHint: jsonschema.Ptr(false),
		},
	}
	handler := func(ctx context.Context, _ *mcp.CallToolRequest, input GetProjectByNameInput) (
		*mcp.CallToolResult, any, error) {
		response, err := p.client.Project().Get(input.Project)
		if err != nil {
			return nil, nil, fmt.Errorf("error retrieving project '%s': %w", input.Project, err)
		}
		text, err := json.Marshal(response)
		if err != nil {
			return nil, nil, fmt.Errorf("error marshalling project '%s': %w", input.Project, err)
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: string(text),
				},
			},
		}, nil, nil
	}
	return &Tool{
		MCPTool:     tool,
		IsWriteTool: false,
		RegisterWith: func(server *mcp.Server) {
			mcp.AddTool(server, tool, handler)
		},
	
	}
}
type CreateProjectInput struct {
	Project     string `json:"project" jsonschema:"Name of the project to create"`
	DisplayName string `json:"displayName" jsonschema:"Display name for the project"`
	Description string `json:"description" jsonschema:"Description for the project"`
}
func (p *project) Create() *Tool {
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
	return &Tool{
		MCPTool:     tool,
		IsWriteTool: false,
		RegisterWith: func(server *mcp.Server) {
			mcp.AddTool(server, tool, handler)
		},
	
	}
}
