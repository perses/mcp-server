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
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/perses/mcp-server/pkg/tools"
	"github.com/perses/mcp-server/pkg/tools/internal/dashboardutil"
)

const (
	scopeProject = "project"
	scopeGlobal  = "global"
)

// resourceKinds describes a resource that exists in a project and a global flavor, e.g. Datasource and GlobalDatasource.
type resourceKinds struct {
	subject string
	project string
	global  string
}

// decodedResource is a datasource or variable argument decoded as JSON, with its scope resolved.
type decodedResource struct {
	scope   string
	project string
	name    string
	// document is the resource JSON with kind and metadata.project filled.
	document []byte
}

// decodeResource decodes a JSON object (or a JSON document serialized in a string) and resolves its scope from, in
// order: kind, the scope argument, then the project argument or metadata.project. Problems are returned as issues.
func decodeResource(raw json.RawMessage, field string, kinds resourceKinds, scope, project string) (*decodedResource, []dashboardutil.Issue) {
	var fields map[string]any
	if issue := dashboardutil.DecodeValue(raw, field, &fields); issue != nil {
		return nil, []dashboardutil.Issue{*issue}
	}
	if fields == nil {
		return nil, []dashboardutil.Issue{{Path: field, Message: "the " + kinds.subject + " must be a JSON object"}}
	}

	metadata, ok := fields["metadata"].(map[string]any)
	if !ok {
		if fields["metadata"] != nil {
			return nil, []dashboardutil.Issue{{Path: "metadata", Message: "metadata must be an object with a name"}}
		}
		metadata = map[string]any{}
		fields["metadata"] = metadata
	}
	metadataProject, _ := metadata["project"].(string)

	var issues []dashboardutil.Issue
	kind, _ := fields["kind"].(string)
	kindScope := ""
	switch kind {
	case "":
	case kinds.project:
		kindScope = scopeProject
	case kinds.global:
		kindScope = scopeGlobal
	default:
		return nil, []dashboardutil.Issue{{
			Path:    "kind",
			Message: fmt.Sprintf("invalid kind %q", kind),
			Hint:    fmt.Sprintf("kind must be %q or %q", kinds.project, kinds.global),
		}}
	}
	switch {
	case kindScope != "" && scope != "" && kindScope != scope:
		return nil, []dashboardutil.Issue{{
			Path:    "kind",
			Message: fmt.Sprintf("kind %q does not match scope %q", kind, scope),
			Hint:    "omit scope: it is taken from kind",
		}}
	case kindScope != "":
		scope = kindScope
	case scope == "" && (project != "" || metadataProject != ""):
		scope = scopeProject
	case scope == "":
		return nil, []dashboardutil.Issue{{
			Message: fmt.Sprintf("cannot tell whether this is a project or a global %s", kinds.subject),
			Hint:    fmt.Sprintf("set kind (%q or %q), scope or project", kinds.project, kinds.global),
		}}
	}
	if scope == scopeProject {
		fields["kind"] = kinds.project
	} else {
		fields["kind"] = kinds.global
	}

	name, _ := metadata["name"].(string)
	if name == "" {
		issues = append(issues, dashboardutil.Issue{Path: "metadata.name", Message: "the " + kinds.subject + " name is required"})
	}
	if scope == scopeProject {
		switch {
		case metadataProject == "" && project == "":
			issues = append(issues, dashboardutil.Issue{
				Path:    "metadata.project",
				Message: "the project is required",
				Hint:    "pass the project argument",
			})
		case metadataProject == "":
			metadata["project"] = project
		case project != "" && !strings.EqualFold(metadataProject, project):
			issues = append(issues, dashboardutil.Issue{
				Path:    "metadata.project",
				Message: fmt.Sprintf("metadata.project '%s' does not match project '%s'", metadataProject, project),
				Hint:    "omit metadata.project: it is filled with the project argument",
			})
		default:
			project = metadataProject
		}
	} else {
		project = ""
	}
	if len(issues) > 0 {
		return nil, issues
	}

	document, err := json.Marshal(fields)
	if err != nil {
		return nil, []dashboardutil.Issue{{Message: fmt.Sprintf("unable to encode the %s: %s", kinds.subject, err)}}
	}
	return &decodedResource{scope: scope, project: project, name: name, document: document}, nil
}

// unmarshalResource decodes the resource into its Perses model, reporting the errors of the model as an issue.
func unmarshalResource(document []byte, target any) []dashboardutil.Issue {
	if err := json.Unmarshal(document, target); err != nil {
		var typeErr *json.UnmarshalTypeError
		if errors.As(err, &typeErr) {
			return []dashboardutil.Issue{{Path: typeErr.Field, Message: fmt.Sprintf("expected %s, got %s", typeErr.Type, typeErr.Value)}}
		}
		return []dashboardutil.Issue{{Message: err.Error()}}
	}
	return nil
}

// writeResult encodes a validation result as the text content of the tool result.
func writeResult(result any) (string, error) {
	text, err := dashboardutil.MarshalText(result)
	if err != nil {
		return "", fmt.Errorf("error marshalling validation result: %w", err)
	}
	return text, nil
}

func projectProperty(description string) *jsonschema.Schema {
	return &jsonschema.Schema{
		Type:        tools.SchemaTypeString,
		Description: description,
		MinLength:   new(1),
		MaxLength:   new(75),
		Pattern:     tools.PatternResourceName,
	}
}

func scopeProperty(kinds resourceKinds) *jsonschema.Schema {
	return &jsonschema.Schema{
		Type: tools.SchemaTypeString,
		Description: fmt.Sprintf("Optional: taken from kind (%s or %s) when set, else project when a project is given.",
			kinds.project, kinds.global),
		Enum: []any{scopeProject, scopeGlobal},
	}
}
