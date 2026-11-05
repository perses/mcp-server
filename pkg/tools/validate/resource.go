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

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/perses/mcp-server/pkg/tools"
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

func projectProperty(description string) *jsonschema.Schema {
	return &jsonschema.Schema{
		Type:        tools.SchemaTypeString,
		Description: description,
		MinLength:   new(1),
		MaxLength:   new(75),
		Pattern:     tools.PatternResourceName,
	}
}

// kindOf returns the kind of a datasource or variable, which tells whether it is a project or a global resource. A
// decoding error is left to the Perses model, which also rejects a missing or unknown kind.
func kindOf(raw json.RawMessage) string {
	var header struct {
		Kind string `json:"kind"`
	}
	_ = json.Unmarshal(raw, &header)
	return header.Kind
}
