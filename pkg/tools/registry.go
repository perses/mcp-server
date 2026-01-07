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

import apiClient "github.com/perses/perses/pkg/client/api/v1"

// ToolRegistryInterface provides access to all resource tool interfaces
type ToolRegistryInterface interface {
	Dashboard() DashboardInterface
	Project() ProjectInterface
	Datasource() DatasourceInterface
	GlobalDatasource() GlobalDatasourceInterface
	Role() RoleInterface
	GlobalRole() GlobalRoleInterface
	RoleBinding() RoleBindingInterface
	GlobalRoleBinding() GlobalRoleBindingInterface
	Variable() VariableInterface
	GlobalVariable() GlobalVariableInterface
	Plugin() PluginInterface

	// GetAllTools returns all tools from all resources
	GetAllTools() []*Tool
}

type toolRegistry struct {
	ToolRegistryInterface
	client apiClient.ClientInterface
}

// NewToolRegistry creates a new tool registry instance
func NewToolRegistry(client apiClient.ClientInterface) ToolRegistryInterface {
	return &toolRegistry{
		client: client,
	}
}

func (r *toolRegistry) Dashboard() DashboardInterface {
	return newDashboard(r.client)
}

func (r *toolRegistry) Project() ProjectInterface {
	return newProject(r.client)
}

func (r *toolRegistry) Datasource() DatasourceInterface {
	return newDatasource(r.client)
}

func (r *toolRegistry) GlobalDatasource() GlobalDatasourceInterface {
	return newGlobalDatasource(r.client)
}

func (r *toolRegistry) Role() RoleInterface {
	return newRole(r.client)
}

func (r *toolRegistry) GlobalRole() GlobalRoleInterface {
	return newGlobalRole(r.client)
}

func (r *toolRegistry) RoleBinding() RoleBindingInterface {
	return newRoleBinding(r.client)
}

func (r *toolRegistry) GlobalRoleBinding() GlobalRoleBindingInterface {
	return newGlobalRoleBinding(r.client)
}

func (r *toolRegistry) Variable() VariableInterface {
	return newVariable(r.client)
}

func (r *toolRegistry) GlobalVariable() GlobalVariableInterface {
	return newGlobalVariable(r.client)
}

func (r *toolRegistry) Plugin() PluginInterface {
	return newPlugin(r.client)
}

// GetAllTools collects all tools from all resource interfaces
func (r *toolRegistry) GetAllTools() []*Tool {
	var allTools []*Tool

	allTools = append(allTools, r.Dashboard().GetTools()...)
	allTools = append(allTools, r.Project().GetTools()...)
	allTools = append(allTools, r.Datasource().GetTools()...)
	allTools = append(allTools, r.GlobalDatasource().GetTools()...)
	allTools = append(allTools, r.Role().GetTools()...)
	allTools = append(allTools, r.GlobalRole().GetTools()...)
	allTools = append(allTools, r.RoleBinding().GetTools()...)
	allTools = append(allTools, r.GlobalRoleBinding().GetTools()...)
	allTools = append(allTools, r.Variable().GetTools()...)
	allTools = append(allTools, r.GlobalVariable().GetTools()...)
	allTools = append(allTools, r.Plugin().GetTools()...)

	return allTools
}
