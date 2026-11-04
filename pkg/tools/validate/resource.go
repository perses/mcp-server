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
	"fmt"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/perses/mcp-server/pkg/tools"
	"github.com/perses/mcp-server/pkg/tools/internal/dashboardutil"
)

const (
	scopeProject = "project"
	scopeGlobal  = "global"
)

// result is the result of the validation tools. Error is set when the resource cannot be decoded.
type result struct {
	Valid bool   `json:"valid"`
	Error string `json:"error,omitempty"`
}

// invalid is the result of a resource that Perses rejects, when decoding it or when validating it.
func invalid(err error) *result {
	return &result{Error: err.Error()}
}

// serverResult converts the response of the Perses validation API. An error, including a validation failure, is
// returned unchanged as a tool error.
func serverResult(err error) (*result, error) {
	if err != nil {
		return nil, err
	}
	return &result{Valid: true}, nil
}

// resourceKinds describes a resource that exists in a project and a global flavor, e.g. Datasource and GlobalDatasource.
type resourceKinds struct {
	project string
	global  string
}

func (k resourceKinds) of(scope string) string {
	if scope == scopeProject {
		return k.project
	}
	return k.global
}

// resourceHeader is the part of a datasource or variable needed to know which Perses API validates it.
type resourceHeader struct {
	Kind     string `json:"kind"`
	Metadata struct {
		Project string `json:"project"`
	} `json:"metadata"`
}

// resolveScope resolves the scope of a datasource or variable from, in order: kind, the scope argument, then the
// project argument or metadata.project. An unknown kind is left to the Perses model to reject.
func resolveScope(raw json.RawMessage, kinds resourceKinds, scope, project string) (string, error) {
	if scope != "" && scope != scopeProject && scope != scopeGlobal {
		return "", fmt.Errorf("invalid scope %q: use %q or %q", scope, scopeProject, scopeGlobal)
	}
	var header resourceHeader
	// A decoding error is reported by the Perses model when decoding the resource.
	_ = json.Unmarshal(raw, &header)
	kindScope := ""
	switch header.Kind {
	case kinds.project:
		kindScope = scopeProject
	case kinds.global:
		kindScope = scopeGlobal
	}
	switch {
	case kindScope != "" && scope != "" && kindScope != scope:
		return "", fmt.Errorf("kind %q does not match scope %q", header.Kind, scope)
	case kindScope != "":
		return kindScope, nil
	case scope != "":
		return scope, nil
	case project != "" || header.Metadata.Project != "":
		return scopeProject, nil
	}
	return "", fmt.Errorf("cannot tell whether this is a project or a global resource: set kind (%q or %q), scope or project",
		kinds.project, kinds.global)
}

// decodeResource decodes the resource into its Perses model, kind defaulting to the kind of the scope. The decoding
// error is returned unchanged.
func decodeResource(raw json.RawMessage, kinds resourceKinds, scope string, target any) error {
	document, err := dashboardutil.WithDefaultKind(raw, kinds.of(scope))
	if err != nil {
		return err
	}
	return json.Unmarshal(document, target)
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
