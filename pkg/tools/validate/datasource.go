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

package validate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/perses/mcp-server/pkg/tools"
	v1 "github.com/perses/perses/pkg/model/api/v1"
	datasourceSpec "github.com/perses/spec/go/datasource"
)

var datasourceKinds = resourceKinds{project: string(v1.KindDatasource), global: string(v1.KindGlobalDatasource)}

type ValidateDatasourceInput struct {
	Datasource        json.RawMessage `json:"datasource,omitempty"`
	Name              string          `json:"name,omitempty"`
	Scope             string          `json:"scope,omitempty"`
	Project           string          `json:"project,omitempty"`
	CheckConnectivity bool            `json:"check_connectivity,omitempty"`
}

// datasourceValidation is the result of perses_validate_datasource. Warnings flag configurations that Perses
// accepts but that likely do not work.
type datasourceValidation struct {
	result
	Warnings     []string      `json:"warnings,omitempty"`
	Connectivity *connectivity `json:"connectivity,omitempty"`
}

func (v *validation) Datasource() *tools.Tool {
	tool := &mcp.Tool{
		Name: "perses_validate_datasource",
		Description: "Validate a project or global datasource without saving it, or check a saved one by name. " +
			"Returns {valid, error, warnings, connectivity}, or the error of the Perses server. Warnings flag " +
			"configs that are valid but likely do not work (directUrl only, localhost proxy URL, blocked endpoints, " +
			"another default datasource, name hiding a global datasource). check_connectivity:true queries the " +
			"datasource through the Perses server proxy (nothing is saved).",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Validates a datasource and checks its connectivity",
			ReadOnlyHint:    true,
			DestructiveHint: new(false),
			IdempotentHint:  true,
			OpenWorldHint:   new(false),
		},
		InputSchema: &jsonschema.Schema{
			Type: tools.SchemaTypeObject,
			Properties: map[string]*jsonschema.Schema{
				"datasource": {
					Type:        tools.SchemaTypeObject,
					Description: "Datasource to validate, as a JSON object. Exclusive with name.",
				},
				tools.ResourceName: {
					Type:        tools.SchemaTypeString,
					Description: "Name of a saved datasource to check. Exclusive with datasource.",
					MinLength:   new(1),
					MaxLength:   new(75),
					Pattern:     tools.PatternResourceName,
				},
				"scope":                       scopeProperty(datasourceKinds),
				string(tools.ProjectResource): projectProperty("Project of a project datasource; fills metadata.project when omitted."),
				"check_connectivity": {
					Type:        tools.SchemaTypeBoolean,
					Description: "Also query the datasource through the Perses server (default false).",
				},
			},
		},
	}

	handler := func(ctx context.Context, _ *mcp.CallToolRequest, input ValidateDatasourceInput) (*mcp.CallToolResult, any, error) { //nolint:unparam
		result, err := v.validateDatasource(ctx, input)
		if err != nil {
			return nil, nil, err
		}
		text, err := writeResult(result)
		if err != nil {
			return nil, nil, err
		}
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: text}}}, nil, nil
	}

	return &tools.Tool{
		MCPTool:      tool,
		IsWriteTool:  false,
		ResourceType: tools.ValidateResource,
		RegisterWith: func(server *mcp.Server) { mcp.AddTool(server, tool, handler) },
	}
}

func (v *validation) validateDatasource(ctx context.Context, input ValidateDatasourceInput) (*datasourceValidation, error) {
	hasDatasource := len(input.Datasource) > 0 && string(input.Datasource) != "null"
	if hasDatasource == (input.Name != "") {
		return nil, errors.New("pass either datasource or name")
	}

	var ref datasourceRef
	var validation *result
	var err error
	if hasDatasource {
		ref, validation, err = v.validateDatasourceDocument(input)
	} else {
		ref, err = v.getSavedDatasource(input)
		validation = &result{Valid: true}
	}
	if err != nil {
		return nil, err
	}
	output := &datasourceValidation{result: *validation}
	if ref.name == "" {
		return output, nil
	}

	output.Warnings = append(lintDatasource(ref.spec), v.checkOtherDatasources(ref)...)
	if input.CheckConnectivity {
		if !output.Valid && !ref.saved {
			output.Connectivity = &connectivity{Status: statusSkipped, Message: "fix the error first"}
		} else {
			output.Connectivity = v.checkConnectivity(ctx, ref)
		}
	}
	return output, nil
}

// validateDatasourceDocument validates an unsaved datasource with the Perses API. The returned reference is empty
// when the datasource cannot be decoded.
func (v *validation) validateDatasourceDocument(input ValidateDatasourceInput) (datasourceRef, *result, error) {
	scope, err := resolveScope(input.Datasource, datasourceKinds, input.Scope, input.Project)
	if err != nil {
		return datasourceRef{}, nil, err
	}
	if scope == scopeProject {
		entity := &v1.Datasource{}
		if err := decodeResource(input.Datasource, datasourceKinds, scope, entity); err != nil {
			return datasourceRef{}, invalid(err), nil
		}
		if entity.Metadata.Project == "" {
			entity.Metadata.Project = input.Project
		}
		ref := datasourceRef{scope: scope, project: entity.Metadata.Project, name: entity.Metadata.Name, spec: entity.Spec}
		validation, err := serverResult(v.client.Validate().Datasource(entity))
		return ref, validation, err
	}
	entity := &v1.GlobalDatasource{}
	if err := decodeResource(input.Datasource, datasourceKinds, scope, entity); err != nil {
		return datasourceRef{}, invalid(err), nil
	}
	ref := datasourceRef{scope: scope, name: entity.Metadata.Name, spec: entity.Spec}
	validation, err := serverResult(v.client.Validate().GlobalDatasource(entity))
	return ref, validation, err
}

// getSavedDatasource reads a saved datasource, to check its configuration and connectivity.
func (v *validation) getSavedDatasource(input ValidateDatasourceInput) (datasourceRef, error) {
	scope := input.Scope
	switch {
	case scope != "" && scope != scopeProject && scope != scopeGlobal:
		return datasourceRef{}, fmt.Errorf("invalid scope %q: use %q or %q", scope, scopeProject, scopeGlobal)
	case scope == "" && input.Project != "":
		scope = scopeProject
	case scope == "":
		scope = scopeGlobal
	}
	ref := datasourceRef{scope: scope, project: input.Project, name: input.Name, saved: true}
	if scope == scopeProject {
		if input.Project == "" {
			return datasourceRef{}, errors.New("project is required for a project datasource")
		}
		entity, err := v.client.V1().Datasource(input.Project).Get(input.Name)
		if err != nil {
			return datasourceRef{}, fmt.Errorf("error getting the project datasource %q: %w", input.Name, err)
		}
		ref.spec = entity.Spec
		return ref, nil
	}
	entity, err := v.client.V1().GlobalDatasource().Get(input.Name)
	if err != nil {
		return datasourceRef{}, fmt.Errorf("error getting the global datasource %q: %w", input.Name, err)
	}
	ref.spec = entity.Spec
	return ref, nil
}

// checkOtherDatasources compares the datasource with the saved ones. Two defaults of the same plugin kind in a scope
// are rejected on save, which the validation API does not check. A project datasource hides the global datasource
// of the same name in its project. The checks are best effort: they are skipped when the list cannot be read.
func (v *validation) checkOtherDatasources(ref datasourceRef) []string {
	var warnings []string
	if ref.spec.Default {
		sameName := v.nameComparator()
		for _, other := range v.listDatasourceSpecs(ref) {
			if !sameName(other.name, ref.name) && other.spec.Default && other.spec.Plugin.Kind == ref.spec.Plugin.Kind {
				warnings = append(warnings, fmt.Sprintf("%q is already the default %s in this scope: saving fails while both are default",
					other.name, ref.spec.Plugin.Kind))
				break
			}
		}
	}
	if ref.scope == scopeProject {
		if _, err := v.client.V1().GlobalDatasource().Get(ref.name); err == nil {
			warnings = append(warnings, fmt.Sprintf("the name hides the global datasource %q in this project", ref.name))
		}
	}
	return warnings
}

// nameComparator returns how the Perses server compares resource names: unless its database is case-sensitive, it
// stores the names in lower case, so "Prom" and "prom" are the same datasource. The server configuration is read to
// know it; when it cannot be read, the default of Perses (case-insensitive) is assumed.
func (v *validation) nameComparator() func(a, b string) bool {
	caseSensitive := false
	if cfg, err := v.client.Config(); err == nil && cfg != nil {
		switch {
		case cfg.Database.File != nil:
			caseSensitive = cfg.Database.File.CaseSensitive
		case cfg.Database.SQL != nil:
			caseSensitive = cfg.Database.SQL.CaseSensitive
		}
	}
	if caseSensitive {
		return func(a, b string) bool { return a == b }
	}
	return strings.EqualFold
}

type namedDatasourceSpec struct {
	name string
	spec datasourceSpec.Spec
}

func (v *validation) listDatasourceSpecs(ref datasourceRef) []namedDatasourceSpec {
	var specs []namedDatasourceSpec
	if ref.scope == scopeProject {
		list, err := v.client.V1().Datasource(ref.project).List("")
		if err != nil {
			return nil
		}
		for _, item := range list {
			specs = append(specs, namedDatasourceSpec{name: item.Metadata.Name, spec: item.Spec})
		}
		return specs
	}
	list, err := v.client.V1().GlobalDatasource().List("")
	if err != nil {
		return nil
	}
	for _, item := range list {
		specs = append(specs, namedDatasourceSpec{name: item.Metadata.Name, spec: item.Spec})
	}
	return specs
}
