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
	"github.com/perses/mcp-server/pkg/tools/internal/dashboardutil"
	v1 "github.com/perses/perses/pkg/model/api/v1"
	datasourceSpec "github.com/perses/spec/go/datasource"
)

var datasourceKinds = resourceKinds{subject: "datasource", project: string(v1.KindDatasource), global: string(v1.KindGlobalDatasource)}

const datasourcePluginHint = "use perses_get_plugin_schema with the datasource plugin kind to see the allowed fields"

type ValidateDatasourceInput struct {
	Datasource        json.RawMessage `json:"datasource,omitempty"`
	Name              string          `json:"name,omitempty"`
	Scope             string          `json:"scope,omitempty"`
	Project           string          `json:"project,omitempty"`
	CheckConnectivity bool            `json:"check_connectivity,omitempty"`
}

// datasourceValidation is the result of perses_validate_datasource.
type datasourceValidation struct {
	dashboardutil.Validation
	Connectivity *connectivity `json:"connectivity,omitempty"`
}

func (v *validation) Datasource() *tools.Tool {
	tool := &mcp.Tool{
		Name: "perses_validate_datasource",
		Description: "Validate a project or global datasource without saving it, or check a saved one by name. " +
			"Returns {valid, errors, warnings, connectivity}: errors and warnings have the JSON path of the field, a " +
			"message and often a hint. Warnings flag configs that are valid but likely do not work (directUrl only, " +
			"localhost proxy URL, blocked endpoints, name hiding a global datasource). check_connectivity:true queries " +
			"the datasource through the Perses server proxy (nothing is saved).",
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
					Types:       []string{tools.SchemaTypeObject, tools.SchemaTypeString},
					Description: "Datasource to validate, as a JSON object (preferred) or a JSON string. Exclusive with name.",
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
	if input.Scope != "" && input.Scope != scopeProject && input.Scope != scopeGlobal {
		return nil, fmt.Errorf("invalid scope %q: use %q or %q", input.Scope, scopeProject, scopeGlobal)
	}

	var ref datasourceRef
	var errs []dashboardutil.Issue
	var err error
	if hasDatasource {
		ref, errs, err = v.validateDatasourceDocument(input)
	} else {
		ref, errs, err = v.getSavedDatasource(input)
	}
	if err != nil || ref.name == "" {
		return &datasourceValidation{Validation: *dashboardutil.NewValidation(errs, nil)}, err
	}

	warnings := lintDatasource(ref.spec)
	conflicts, shadowing := v.checkOtherDatasources(ref)
	errs = append(errs, conflicts...)
	warnings = append(warnings, shadowing...)

	result := &datasourceValidation{Validation: *dashboardutil.NewValidation(errs, warnings)}
	if input.CheckConnectivity {
		if len(errs) > 0 && !ref.saved {
			result.Connectivity = &connectivity{Status: statusSkipped, Message: "fix the errors first"}
		} else {
			result.Connectivity = v.checkConnectivity(ctx, ref)
		}
	}
	return result, nil
}

// validateDatasourceDocument validates an unsaved datasource with the Perses API.
func (v *validation) validateDatasourceDocument(input ValidateDatasourceInput) (datasourceRef, []dashboardutil.Issue, error) {
	decoded, issues := decodeResource(input.Datasource, "datasource", datasourceKinds, input.Scope, input.Project)
	if len(issues) > 0 {
		return datasourceRef{}, issues, nil
	}
	ref := datasourceRef{scope: decoded.scope, project: decoded.project, name: decoded.name}
	var validateErr error
	if decoded.scope == scopeProject {
		entity := &v1.Datasource{}
		if issues := unmarshalResource(decoded.document, entity); len(issues) > 0 {
			return datasourceRef{}, issues, nil
		}
		ref.spec = entity.Spec
		validateErr = v.client.Validate().Datasource(entity)
	} else {
		entity := &v1.GlobalDatasource{}
		if issues := unmarshalResource(decoded.document, entity); len(issues) > 0 {
			return datasourceRef{}, issues, nil
		}
		ref.spec = entity.Spec
		validateErr = v.client.Validate().GlobalDatasource(entity)
	}
	if validateErr != nil {
		message, ok := dashboardutil.IsBadRequest(validateErr)
		if !ok {
			return datasourceRef{}, nil, fmt.Errorf("error validating the %s datasource: %w", decoded.scope, validateErr)
		}
		return ref, dashboardutil.ResourceIssuesFromServerMessage(message, "spec.plugin", datasourcePluginHint), nil
	}
	return ref, nil, nil
}

// getSavedDatasource reads a saved datasource, to check its configuration and connectivity.
func (v *validation) getSavedDatasource(input ValidateDatasourceInput) (datasourceRef, []dashboardutil.Issue, error) {
	scope := input.Scope
	if scope == "" {
		scope = scopeGlobal
		if input.Project != "" {
			scope = scopeProject
		}
	}
	ref := datasourceRef{scope: scope, project: input.Project, saved: true}
	var err error
	if scope == scopeProject {
		if input.Project == "" {
			return datasourceRef{}, nil, errors.New("project is required for a project datasource")
		}
		var entity *v1.Datasource
		if entity, err = v.client.V1().Datasource(input.Project).Get(input.Name); err == nil {
			ref.spec = entity.Spec
			ref.name = entity.Metadata.Name
		}
	} else {
		var entity *v1.GlobalDatasource
		if entity, err = v.client.V1().GlobalDatasource().Get(input.Name); err == nil {
			ref.spec = entity.Spec
			ref.name = entity.Metadata.Name
		}
	}
	if err != nil {
		if dashboardutil.IsNotFound(err) {
			hint := "list the datasources with perses_list_global_datasources"
			if scope == scopeProject {
				hint = "list the datasources with perses_list_project_datasources, or set scope to global"
			}
			return datasourceRef{}, []dashboardutil.Issue{{
				Path:    tools.ResourceName,
				Message: fmt.Sprintf("%s datasource %q not found", scope, input.Name),
				Hint:    hint,
			}}, nil
		}
		return datasourceRef{}, nil, fmt.Errorf("error getting the %s datasource %q: %w", scope, input.Name, err)
	}
	if ref.name == "" {
		ref.name = input.Name
	}
	return ref, nil, nil
}

// checkOtherDatasources compares the datasource with the saved ones. Two defaults of the same plugin kind in a scope
// are rejected on save, which the validation API does not check. A project datasource hides the global datasource
// of the same name in its project. The checks are best effort: they are skipped when the list cannot be read.
func (v *validation) checkOtherDatasources(ref datasourceRef) ([]dashboardutil.Issue, []dashboardutil.Issue) {
	var errs, warnings []dashboardutil.Issue
	if ref.spec.Default {
		sameName := v.nameComparator()
		for _, other := range v.listDatasourceSpecs(ref) {
			if !sameName(other.name, ref.name) && other.spec.Default && other.spec.Plugin.Kind == ref.spec.Plugin.Kind {
				errs = append(errs, dashboardutil.Issue{
					Path:    "spec.default",
					Message: fmt.Sprintf("%q is already the default %s in this scope; saving fails", other.name, ref.spec.Plugin.Kind),
					Hint:    fmt.Sprintf("set default to false, or unset default on %q first", other.name),
				})
				break
			}
		}
	}
	if ref.scope == scopeProject {
		if _, err := v.client.V1().GlobalDatasource().Get(ref.name); err == nil {
			warnings = append(warnings, dashboardutil.Issue{
				Path:    "metadata.name",
				Message: fmt.Sprintf("hides the global datasource %q in this project", ref.name),
				Hint:    "use another name unless the project must override the global datasource",
			})
		}
	}
	return errs, warnings
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
