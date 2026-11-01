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

// Package dashboardutil contains the dashboard logic shared by the MCP tools: decoding the dashboard argument,
// local checks, structured validation errors, compact summaries and grid layout placement.
package dashboardutil

import (
	"bytes"
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
)

// Issue is a problem located in a dashboard, precise enough for a caller to fix a single field.
type Issue struct {
	// Path is the JSON path of the offending value in the dashboard, e.g. spec.panels.cpu.spec.plugin.spec.calculation.
	// It is empty when the problem cannot be located.
	Path    string `json:"path,omitempty"`
	Message string `json:"message"`
	// Hint suggests how to fix the problem.
	Hint string `json:"hint,omitempty"`
}

func (i Issue) String() string {
	var b strings.Builder
	if i.Path != "" {
		b.WriteString(i.Path)
		b.WriteString(": ")
	}
	b.WriteString(i.Message)
	if i.Hint != "" {
		b.WriteString(" (")
		b.WriteString(i.Hint)
		b.WriteString(")")
	}
	return b.String()
}

// resourceNamePattern is the pattern of the Perses resource names, also used for panel keys.
var resourceNamePattern = regexp.MustCompile(`^[a-zA-Z0-9_.-]+$`)

// IsValidKey reports whether a panel key is acceptable as a Perses resource name.
func IsValidKey(key string) bool {
	return resourceNamePattern.MatchString(key)
}

// Validation is the result of validating a dashboard. Warnings do not make the dashboard invalid: they flag
// mistakes that Perses accepts but that give unexpected results, e.g. a panel that no layout displays.
type Validation struct {
	Valid    bool    `json:"valid"`
	Errors   []Issue `json:"errors,omitempty"`
	Warnings []Issue `json:"warnings,omitempty"`
}

// NewValidation builds a validation result from the errors and warnings found.
func NewValidation(errs, warnings []Issue) *Validation {
	return &Validation{Valid: len(errs) == 0, Errors: errs, Warnings: warnings}
}

// MarshalText encodes a tool result as compact JSON without HTML escaping, so that hints such as "<key>" or
// PromQL comparisons stay readable by the model.
func MarshalText(value any) (string, error) {
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return "", err
	}
	return strings.TrimSuffix(buf.String(), "\n"), nil
}

// Error formats the errors as a single message, for tools that fail instead of returning the validation.
func (v *Validation) Error() string {
	messages := make([]string, 0, len(v.Errors))
	for _, issue := range v.Errors {
		messages = append(messages, issue.String())
	}
	return strings.Join(messages, "; ")
}

// identifierPattern matches the map keys that can be written in dot notation in a JSON path.
var identifierPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_-]*$`)

// keyPath is the JSON path of the value at key in the map at parent. Keys that are not plain identifiers (e.g.
// "cpu.load") are written in bracket notation, e.g. spec.panels["cpu.load"], so that the path stays unambiguous.
func keyPath(parent, key string) string {
	if identifierPattern.MatchString(key) {
		return parent + "." + key
	}
	return parent + "[" + strconv.Quote(key) + "]"
}

// panelPath is the JSON path of the panel with the given key.
func panelPath(key string) string {
	return keyPath("spec.panels", key)
}

// pluginSpecPath builds the JSON path of a plugin field from the plugin path and the CUE path reported by Perses,
// which is relative to the plugin (e.g. spec.calculation). An empty CUE path designates the plugin itself.
func pluginSpecPath(pluginPath, cuePath string) string {
	if cuePath == "" {
		return pluginPath
	}
	return pluginPath + "." + cuePath
}
