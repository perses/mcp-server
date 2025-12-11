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
)

type ListNewDashboardsInput struct {
	Project string `json:"project" jsonschema:"Project name to list dashboards from"`
}

func ListDashboards(client apiClient.ClientInterface) (*newMcp.Tool, newMcp.ToolHandlerFor[ListNewDashboardsInput, any]) {

	tool := &newMcp.Tool{
		Name:        "perses_list_dashboards",
		Description: "List dashboards for a specific project",
		Annotations: &newMcp.ToolAnnotations{
			DestructiveHint: jsonschema.Ptr(false),
			IdempotentHint:  true,
			OpenWorldHint:   jsonschema.Ptr(false),
			ReadOnlyHint:    true,
			Title:           "List dashboards for a specific project in Perses",
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
	handler := func(ctx context.Context, _ *newMcp.CallToolRequest, input ListNewDashboardsInput) (result *newMcp.CallToolResult, output any, _ error) {
		response, err := client.Dashboard(input.Project).List("")
		if err != nil {
			return nil, nil, fmt.Errorf("error retrieving dashboards: %w", err)
		}

		text, err := json.Marshal(response)
		if err != nil {
			return nil, nil, fmt.Errorf("error marshalling dashboards: %w", err)
		}

		return &newMcp.CallToolResult{
			Content: []newMcp.Content{
				&newMcp.TextContent{
					Text: string(text),
				},
			},
		}, nil, nil
	}
	return tool, handler
}

type GetDashboardByNameInput struct {
	Project string `json:"project" jsonschema:"Project name to retrieve the dashboard from"`
	Name    string `json:"name" jsonschema:"Dashboard name to retrieve"`
}

func GetDashboardByName(client apiClient.ClientInterface) (*newMcp.Tool, newMcp.ToolHandlerFor[GetDashboardByNameInput, any]) {

	tool := &newMcp.Tool{
		Name:        "perses_get_dashboard_by_name",
		Description: "Get a dashboard by name in a specific project",
		Annotations: &newMcp.ToolAnnotations{
			DestructiveHint: jsonschema.Ptr(false),
			IdempotentHint:  true,
			OpenWorldHint:   jsonschema.Ptr(false),
			ReadOnlyHint:    true,
			Title:           "Gets a dashboard by name in a specific project in Perses",
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
					Description: "Dashboard name",
					MinLength:   jsonschema.Ptr(1),
					MaxLength:   jsonschema.Ptr(75),
					Pattern:     "^[a-zA-Z0-9_.-]+$",
				},
			},
			Required: []string{"project", "name"},
		},
	}
	handler := func(ctx context.Context, _ *newMcp.CallToolRequest, input GetDashboardByNameInput) (result *newMcp.CallToolResult, output any, _ error) {

		response, err := client.Dashboard(input.Project).Get(input.Name)
		if err != nil {
			return nil, nil, fmt.Errorf("error retrieving dashboard '%s' in project '%s': %w", input.Name, input.Project, err)
		}
		return nil, response, nil
	}
	return tool, handler
}

type CreateDashboardInput struct {
	Project   string `json:"project" jsonschema:"Project name to create the dashboard in"`
	Dashboard string `json:"dashboard" jsonschema:"Dashboard JSON as string"`
}

func CreateNewDashboard(client apiClient.ClientInterface) (*newMcp.Tool, newMcp.ToolHandlerFor[CreateDashboardInput, any]) {
	tool := &newMcp.Tool{
		Name:        "perses_create_dashboard",
		Description: "Create a new dashboard in a specific project",
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
				"dashboard": {
					Type:        "string",
					Description: "Dashboard JSON as string",
				},
			},
			Required: []string{"project", "dashboard"},
		},
		Annotations: &newMcp.ToolAnnotations{
			DestructiveHint: jsonschema.Ptr(false),
			IdempotentHint:  true,
			OpenWorldHint:   jsonschema.Ptr(false),
			ReadOnlyHint:    false,
			Title:           "Creates a new dashboard in a specific project in Perses",
		},
	}
	handler := func(ctx context.Context, _ *newMcp.CallToolRequest, input CreateDashboardInput) (*newMcp.CallToolResult, any, error) {

		var dashboard v1.Dashboard
		if err := json.Unmarshal([]byte(input.Dashboard), &dashboard); err != nil {
			return nil, nil, fmt.Errorf("invalid dashboard JSON: %w", err)
		}

		createdDashboard, err := client.Dashboard(input.Project).Create(&dashboard)
		if err != nil {
			return nil, nil, fmt.Errorf("error creating dashboard in project '%s': %w", input.Project, err)
		}

		dashboardJSON, err := json.Marshal(createdDashboard)
		if err != nil {
			return nil, nil, fmt.Errorf("error marshalling created dashboard: %w", err)
		}
		return &newMcp.CallToolResult{
			Content: []newMcp.Content{
				&newMcp.TextContent{
					Text: string(dashboardJSON),
				},
			},
		}, nil, nil
	}
	return tool, handler
}

func CreateDashboard(client apiClient.ClientInterface) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("perses_create_dashboard",
			mcp.WithDescription("Create a new dashboard in a specific project"),
			mcp.WithString("project", mcp.Required(),
				mcp.Description("Project name")),
			mcp.WithString("dashboard", mcp.Required(),
				mcp.Description("Dashboard JSON as string")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:           "Creates a new dashboard in a specific project in Perses",
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
			dashboardStr, err := request.RequireString("dashboard")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			var dashboard v1.Dashboard
			if err := json.Unmarshal([]byte(dashboardStr), &dashboard); err != nil {
				return nil, fmt.Errorf("invalid dashboard JSON: %w", err)
			}

			created, err := client.Dashboard(project).Create(&dashboard)
			if err != nil {
				return nil, fmt.Errorf("error creating dashboard in project '%s': %w", project, err)
			}

			resultJSON, err := json.Marshal(created)
			if err != nil {
				return nil, fmt.Errorf("error marshalling created dashboard: %w", err)
			}

			return mcp.NewToolResultText(string(resultJSON)), nil
		}
}
