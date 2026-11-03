// Copyright The Perses Authors
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

package ephemeraldashboard

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/perses/mcp-server/pkg/tools"
	"github.com/perses/mcp-server/pkg/tools/resource"
	apiClient "github.com/perses/perses/pkg/client/api/v1"
	"github.com/perses/perses/pkg/client/perseshttp"
	v1 "github.com/perses/perses/pkg/model/api/v1"
)

// disabledMessage is appended to errors that mean the ephemeral dashboard API is turned off on the server.
const disabledMessage = "ephemeral dashboards are disabled on this Perses server (config ephemeral_dashboard.enable=false) for every project; " +
	"do not retry, use a regular dashboard (perses_create_dashboard) instead"

// explainError rewrites the generic API error when its status code reveals that ephemeral dashboards are disabled.
// whenMissing tells whether a 404 can only mean that (list) or may also mean the object does not exist (get, delete).
func explainError(err error, wrapped string, whenMissing bool) error {
	var reqErr *perseshttp.RequestError
	if errors.As(err, &reqErr) {
		switch {
		case reqErr.StatusCode == http.StatusMethodNotAllowed, reqErr.StatusCode == http.StatusNotFound && !whenMissing:
			return fmt.Errorf("%s: %s", wrapped, disabledMessage)
		case reqErr.StatusCode == http.StatusNotFound:
			return fmt.Errorf("%s: not found; either it does not exist (check perses_list_ephemeral_dashboards) or %s", wrapped, disabledMessage)
		}
	}
	return fmt.Errorf("%s: %w", wrapped, err)
}

type ephemeraldashboard struct {
	client apiClient.ClientInterface
}

func New(client apiClient.ClientInterface) resource.Resource {
	return &ephemeraldashboard{
		client: client,
	}
}

func (ed *ephemeraldashboard) GetTools() []*tools.Tool {
	return []*tools.Tool{
		ed.List(),
		ed.Get(),
		ed.Create(),
		ed.Update(),
		ed.Delete(),
	}
}

type ListEphemeralDashboardsInput struct {
	Project string `json:"project" jsonschema:"Project name to list ephemeral dashboards from"`
}

func (ed *ephemeraldashboard) List() *tools.Tool {
	tool := &mcp.Tool{
		Name:        "perses_list_ephemeral_dashboards",
		Description: "List the ephemeral dashboards of a project (temporary dashboards deleted automatically when their spec.ttl expires, e.g. for previews). If ephemeral dashboards are disabled on the server, the error says so explicitly: then stop and use regular dashboards.",
		Annotations: &mcp.ToolAnnotations{
			DestructiveHint: jsonschema.Ptr(false),
			IdempotentHint:  true,
			OpenWorldHint:   jsonschema.Ptr(false),
			ReadOnlyHint:    true,
			Title:           "List ephemeral dashboards for a specific project in Perses",
		},
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"project": {
					Type:        "string",
					Description: "Project name (metadata.name from perses_list_projects)",
					MinLength:   jsonschema.Ptr(1),
					MaxLength:   jsonschema.Ptr(75),
					Pattern:     "^[a-zA-Z0-9_.-]+$",
				},
			},
			Required: []string{"project"},
		},
	}

	handler := func(_ context.Context, _ *mcp.CallToolRequest, input ListEphemeralDashboardsInput) (*mcp.CallToolResult, any, error) { //nolint:unparam
		response, err := ed.client.EphemeralDashboard(input.Project).List("")
		if err != nil {
			return nil, nil, explainError(err, "error retrieving ephemeral dashboards", false)
		}

		text, err := json.Marshal(response)
		if err != nil {
			return nil, nil, fmt.Errorf("error marshalling ephemeral dashboards: %w", err)
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
		ResourceType: tools.EphemeralDashboardResource,
		RegisterWith: func(server *mcp.Server) { mcp.AddTool(server, tool, handler) },
	}
}

type GetEphemeralDashboardByNameInput struct {
	Project string `json:"project" jsonschema:"Project name to retrieve the ephemeral dashboard from"`
	Name    string `json:"name" jsonschema:"Ephemeral dashboard name to retrieve"`
}

func (ed *ephemeraldashboard) Get() *tools.Tool {
	tool := &mcp.Tool{
		Name:        "perses_get_ephemeral_dashboard_by_name",
		Description: "Get the full JSON of one ephemeral dashboard by name, including its spec.ttl. If ephemeral dashboards are disabled on the server, the error says so explicitly: then stop and use regular dashboards.",
		Annotations: &mcp.ToolAnnotations{
			DestructiveHint: jsonschema.Ptr(false),
			IdempotentHint:  true,
			OpenWorldHint:   jsonschema.Ptr(false),
			ReadOnlyHint:    true,
			Title:           "Gets an ephemeral dashboard by name in a specific project in Perses",
		},
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"project": {
					Type:        "string",
					Description: "Project name (metadata.name from perses_list_projects)",
					MinLength:   jsonschema.Ptr(1),
					MaxLength:   jsonschema.Ptr(75),
					Pattern:     "^[a-zA-Z0-9_.-]+$",
				},
				"name": {
					Type:        "string",
					Description: "Ephemeral dashboard name",
					MinLength:   jsonschema.Ptr(1),
					MaxLength:   jsonschema.Ptr(75),
					Pattern:     "^[a-zA-Z0-9_.-]+$",
				},
			},
			Required: []string{"project", "name"},
		},
	}

	handler := func(_ context.Context, _ *mcp.CallToolRequest, input GetEphemeralDashboardByNameInput) (*mcp.CallToolResult, any, error) { //nolint:unparam
		response, err := ed.client.EphemeralDashboard(input.Project).Get(input.Name)
		if err != nil {
			return nil, nil, explainError(err, fmt.Sprintf("error retrieving ephemeral dashboard '%s' in project '%s'", input.Name, input.Project), true)
		}
		return nil, response, nil
	}

	return &tools.Tool{
		MCPTool:      tool,
		IsWriteTool:  false,
		ResourceType: tools.EphemeralDashboardResource,
		RegisterWith: func(server *mcp.Server) { mcp.AddTool(server, tool, handler) },
	}
}

type CreateEphemeralDashboardInput struct {
	Project   string `json:"project" jsonschema:"Project name to create the ephemeral dashboard in"`
	Dashboard string `json:"dashboard" jsonschema:"Complete ephemeral dashboard JSON as a string: {kind: EphemeralDashboard, metadata: {name, project}, spec: {ttl: e.g. 24h, display, panels, layouts, variables, duration}}"`
}

func (ed *ephemeraldashboard) Create() *tools.Tool {
	tool := &mcp.Tool{
		Name: "perses_create_ephemeral_dashboard",
		Description: "Create a temporary dashboard in a project that Perses deletes automatically once spec.ttl (e.g. 1h, 24h) has elapsed; " +
			"use it for previews or throwaway views, and perses_create_dashboard for anything permanent. The spec is a regular dashboard spec plus ttl. " +
			"Returns the created object. If ephemeral dashboards are disabled on the server, the error says so explicitly: do not retry or probe other ephemeral tools. " +
			"Validated against installed plugin CUE schemas. To prevalidate, " +
			"pass perses_validate_dashboard a copy with kind \"Dashboard\" and spec.ttl omitted (TTL is not validated); " +
			"see perses_get_plugin_schema for field definitions.",
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"project": {
					Type:        "string",
					Description: "Project name (metadata.name from perses_list_projects)",
					MinLength:   jsonschema.Ptr(1),
					MaxLength:   jsonschema.Ptr(75),
					Pattern:     "^[a-zA-Z0-9_.-]+$",
				},
				"dashboard": {
					Type:        "string",
					Description: "Complete ephemeral dashboard JSON as a string: {kind: EphemeralDashboard, metadata: {name, project}, spec: {ttl: e.g. 24h, display, panels, layouts, variables, duration}}",
				},
			},
			Required: []string{"project", "dashboard"},
		},
		Annotations: &mcp.ToolAnnotations{
			DestructiveHint: jsonschema.Ptr(false),
			IdempotentHint:  true,
			OpenWorldHint:   jsonschema.Ptr(false),
			ReadOnlyHint:    false,
			Title:           "Creates a new ephemeral dashboard in a specific project in Perses",
		},
	}

	handler := func(_ context.Context, _ *mcp.CallToolRequest, input CreateEphemeralDashboardInput) (*mcp.CallToolResult, any, error) { //nolint:unparam
		var dashboardObj v1.EphemeralDashboard
		if err := json.Unmarshal([]byte(input.Dashboard), &dashboardObj); err != nil {
			return nil, nil, fmt.Errorf("invalid ephemeral dashboard JSON: %w", err)
		}

		createdDashboard, err := ed.client.EphemeralDashboard(input.Project).Create(&dashboardObj)
		if err != nil {
			return nil, nil, explainError(err, fmt.Sprintf("error creating ephemeral dashboard in project '%s'", input.Project), false)
		}

		dashboardJSON, err := json.Marshal(createdDashboard)
		if err != nil {
			return nil, nil, fmt.Errorf("error marshalling created ephemeral dashboard: %w", err)
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: string(dashboardJSON),
				},
			},
		}, nil, nil
	}

	return &tools.Tool{
		MCPTool:      tool,
		IsWriteTool:  true,
		ResourceType: tools.EphemeralDashboardResource,
		RegisterWith: func(server *mcp.Server) { mcp.AddTool(server, tool, handler) },
	}
}

type UpdateEphemeralDashboardInput struct {
	Project   string `json:"project" jsonschema:"Project name to update the ephemeral dashboard in"`
	Dashboard string `json:"dashboard" jsonschema:"Complete ephemeral dashboard JSON as a string: {kind: EphemeralDashboard, metadata: {name, project}, spec: {ttl: e.g. 24h, display, panels, layouts, variables, duration}}"`
}

func (ed *ephemeraldashboard) Update() *tools.Tool {
	tool := &mcp.Tool{
		Name: "perses_update_ephemeral_dashboard",
		Description: "Replace an existing ephemeral dashboard with its complete JSON (read it first with perses_get_ephemeral_dashboard_by_name); " +
			"setting a new spec.ttl extends its lifetime. Returns the updated object. Validated against installed plugin CUE schemas. To prevalidate, " +
			"pass perses_validate_dashboard a copy with kind \"Dashboard\" and spec.ttl omitted (TTL is not validated); " +
			"see perses_get_plugin_schema for field definitions.",
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"project": {
					Type:        "string",
					Description: "Project name (metadata.name from perses_list_projects)",
					MinLength:   jsonschema.Ptr(1),
					MaxLength:   jsonschema.Ptr(75),
					Pattern:     "^[a-zA-Z0-9_.-]+$",
				},
				"dashboard": {
					Type:        "string",
					Description: "Complete ephemeral dashboard JSON as a string: {kind: EphemeralDashboard, metadata: {name, project}, spec: {ttl: e.g. 24h, display, panels, layouts, variables, duration}}",
				},
			},
			Required: []string{"project", "dashboard"},
		},
		Annotations: &mcp.ToolAnnotations{
			DestructiveHint: jsonschema.Ptr(false),
			IdempotentHint:  true,
			OpenWorldHint:   jsonschema.Ptr(false),
			ReadOnlyHint:    false,
			Title:           "Updates an existing ephemeral dashboard in a specific project in Perses",
		},
	}

	handler := func(_ context.Context, _ *mcp.CallToolRequest, input UpdateEphemeralDashboardInput) (*mcp.CallToolResult, any, error) { //nolint:unparam
		var dashboardObj v1.EphemeralDashboard
		if err := json.Unmarshal([]byte(input.Dashboard), &dashboardObj); err != nil {
			return nil, nil, fmt.Errorf("invalid ephemeral dashboard JSON: %w", err)
		}

		updatedDashboard, err := ed.client.EphemeralDashboard(input.Project).Update(&dashboardObj)
		if err != nil {
			return nil, nil, explainError(err, fmt.Sprintf("error updating ephemeral dashboard in project '%s'", input.Project), true)
		}

		dashboardJSON, err := json.Marshal(updatedDashboard)
		if err != nil {
			return nil, nil, fmt.Errorf("error marshalling updated ephemeral dashboard: %w", err)
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: string(dashboardJSON),
				},
			},
		}, nil, nil
	}

	return &tools.Tool{
		MCPTool:      tool,
		IsWriteTool:  true,
		ResourceType: tools.EphemeralDashboardResource,
		RegisterWith: func(server *mcp.Server) { mcp.AddTool(server, tool, handler) },
	}
}

type DeleteEphemeralDashboardInput struct {
	Project string `json:"project" jsonschema:"Project name"`
	Name    string `json:"name" jsonschema:"Ephemeral dashboard name to delete"`
}

func (ed *ephemeraldashboard) Delete() *tools.Tool {
	tool := &mcp.Tool{
		Name:        "perses_delete_ephemeral_dashboard",
		Description: "Delete an ephemeral dashboard before its TTL expires. If ephemeral dashboards are disabled on the server, the error says so explicitly: then stop and use regular dashboards.",
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"project": {
					Type:        "string",
					Description: "Project name (metadata.name from perses_list_projects)",
					MinLength:   jsonschema.Ptr(1),
					MaxLength:   jsonschema.Ptr(75),
					Pattern:     "^[a-zA-Z0-9_.-]+$",
				},
				"name": {
					Type:        "string",
					Description: "Ephemeral dashboard name",
					MinLength:   jsonschema.Ptr(1),
					MaxLength:   jsonschema.Ptr(75),
					Pattern:     "^[a-zA-Z0-9_.-]+$",
				},
			},
			Required: []string{"project", "name"},
		},
		Annotations: &mcp.ToolAnnotations{
			DestructiveHint: jsonschema.Ptr(true),
			IdempotentHint:  true,
			OpenWorldHint:   jsonschema.Ptr(false),
			ReadOnlyHint:    false,
			Title:           "Deletes an ephemeral dashboard from a specific project in Perses",
		},
	}

	handler := func(_ context.Context, _ *mcp.CallToolRequest, input DeleteEphemeralDashboardInput) (*mcp.CallToolResult, any, error) { //nolint:unparam
		err := ed.client.EphemeralDashboard(input.Project).Delete(input.Name)
		if err != nil {
			return nil, nil, explainError(err, fmt.Sprintf("error deleting ephemeral dashboard '%s' in project '%s'", input.Name, input.Project), true)
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: fmt.Sprintf("Ephemeral dashboard '%s' deleted successfully from project '%s'", input.Name, input.Project),
				},
			},
		}, nil, nil
	}

	return &tools.Tool{
		MCPTool:      tool,
		IsWriteTool:  true,
		ResourceType: tools.EphemeralDashboardResource,
		RegisterWith: func(server *mcp.Server) { mcp.AddTool(server, tool, handler) },
	}
}
