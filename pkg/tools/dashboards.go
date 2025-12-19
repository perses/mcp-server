package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	apiClient "github.com/perses/perses/pkg/client/api/v1"
	v1 "github.com/perses/perses/pkg/model/api/v1"
)

type ListNewDashboardsInput struct {
	Project string `json:"project" jsonschema:"Project name to list dashboards from"`
}

func ListDashboards(client apiClient.ClientInterface) (*mcp.Tool, mcp.ToolHandlerFor[ListNewDashboardsInput, any]) {
	tool := &mcp.Tool{
		Name:        "perses_list_dashboards",
		Description: "List dashboards for a specific project",
		Annotations: &mcp.ToolAnnotations{
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

	handler := func(ctx context.Context, _ *mcp.CallToolRequest, input ListNewDashboardsInput) (result *mcp.CallToolResult, output any, _ error) {
		response, err := client.Dashboard(input.Project).List("")
		if err != nil {
			return nil, nil, fmt.Errorf("error retrieving dashboards: %w", err)
		}

		text, err := json.Marshal(response)
		if err != nil {
			return nil, nil, fmt.Errorf("error marshalling dashboards: %w", err)
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
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

func GetDashboardByName(client apiClient.ClientInterface) (*mcp.Tool, mcp.ToolHandlerFor[GetDashboardByNameInput, any]) {
	tool := &mcp.Tool{
		Name:        "perses_get_dashboard_by_name",
		Description: "Get a dashboard by name in a specific project",
		Annotations: &mcp.ToolAnnotations{
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

	handler := func(ctx context.Context, _ *mcp.CallToolRequest, input GetDashboardByNameInput) (result *mcp.CallToolResult, output any, _ error) {
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

func CreateNewDashboard(client apiClient.ClientInterface) (*mcp.Tool, mcp.ToolHandlerFor[CreateDashboardInput, any]) {
	tool := &mcp.Tool{
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
		Annotations: &mcp.ToolAnnotations{
			DestructiveHint: jsonschema.Ptr(false),
			IdempotentHint:  true,
			OpenWorldHint:   jsonschema.Ptr(false),
			ReadOnlyHint:    false,
			Title:           "Creates a new dashboard in a specific project in Perses",
		},
	}

	handler := func(ctx context.Context, _ *mcp.CallToolRequest, input CreateDashboardInput) (*mcp.CallToolResult, any, error) {
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
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: string(dashboardJSON),
				},
			},
		}, nil, nil
	}
	return tool, handler
}
