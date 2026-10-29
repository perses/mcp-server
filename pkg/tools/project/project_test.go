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

package project

import (
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
)

func TestCreateProjectInputSchema(t *testing.T) {
	schema, ok := New(nil).Create().MCPTool.InputSchema.(*jsonschema.Schema)
	if !ok {
		t.Fatal("input schema is not a *jsonschema.Schema")
	}
	resolved, err := schema.Resolve(nil)
	if err != nil {
		t.Fatalf("unable to resolve the input schema: %v", err)
	}

	testSuites := []struct {
		title   string
		input   map[string]any
		isValid bool
	}{
		{
			title:   "only the project name",
			input:   map[string]any{"project": "demo"},
			isValid: true,
		},
		{
			title: "description with spaces and punctuation",
			input: map[string]any{
				"project":     "demo",
				"displayName": "Demo",
				"description": "Dashboards of the demo, owned by the platform team.",
			},
			isValid: true,
		},
		{
			title:   "missing project name",
			input:   map[string]any{"description": "Demo project"},
			isValid: false,
		},
		{
			title:   "invalid project name",
			input:   map[string]any{"project": "my project"},
			isValid: false,
		},
	}
	for _, test := range testSuites {
		t.Run(test.title, func(t *testing.T) {
			err := resolved.Validate(test.input)
			if test.isValid && err != nil {
				t.Errorf("expected the input to be valid, got: %v", err)
			}
			if !test.isValid && err == nil {
				t.Error("expected the input to be invalid")
			}
		})
	}
}
