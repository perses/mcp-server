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

package dashboardutil

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	v1 "github.com/perses/perses/pkg/model/api/v1"
)

// panelRefPrefix is the prefix of the JSON references of layout items to panels.
const panelRefPrefix = "#/spec/panels/"

// Decode decodes the dashboard argument of a tool. The argument is a JSON object or, for backward compatibility, a
// string holding the JSON document. kind defaults to Dashboard and metadata.project to project. Problems are
// returned as issues rather than as an error, each one located as precisely as possible.
func Decode(raw json.RawMessage, project string) (*v1.Dashboard, []Issue) {
	document, issue := unwrap(raw)
	if issue != nil {
		return nil, []Issue{*issue}
	}

	var fields map[string]any
	decoder := json.NewDecoder(bytes.NewReader(document))
	decoder.UseNumber()
	if err := decoder.Decode(&fields); err != nil {
		return nil, []Issue{syntaxIssue(err)}
	}
	trailingStart := decoder.InputOffset()
	var trailing json.RawMessage
	switch err := decoder.Decode(&trailing); {
	case errors.Is(err, io.EOF):
	case err != nil:
		return nil, []Issue{syntaxIssue(err)}
	default:
		return nil, []Issue{{
			Message: fmt.Sprintf("unexpected JSON value after dashboard at byte %d", trailingStart),
			Hint:    "pass exactly one JSON dashboard",
		}}
	}
	if fields == nil {
		return nil, []Issue{{Message: "the dashboard must be a JSON object"}}
	}

	issues := applyDefaults(fields, project)
	issues = append(issues, checkRawReferences(fields)...)
	if len(issues) > 0 {
		return nil, issues
	}

	normalized, err := json.Marshal(fields)
	if err != nil {
		return nil, []Issue{{Message: fmt.Sprintf("unable to encode the dashboard: %s", err)}}
	}
	var dashboard v1.Dashboard
	if err := json.Unmarshal(normalized, &dashboard); err != nil {
		return nil, []Issue{decodeIssue(err)}
	}
	return &dashboard, nil
}

// unwrap returns the JSON document of the dashboard argument, decoding it when it is given as a string.
func unwrap(raw json.RawMessage) ([]byte, *Issue) {
	document := bytes.TrimSpace(raw)
	if len(document) == 0 || bytes.Equal(document, []byte("null")) {
		return nil, &Issue{Message: "the value is required", Hint: "pass the value as a JSON object"}
	}
	if document[0] != '"' {
		return document, nil
	}
	var text string
	if err := json.Unmarshal(document, &text); err != nil {
		return nil, &Issue{Message: fmt.Sprintf("the string cannot be decoded as JSON: %s", err)}
	}
	document = bytes.TrimSpace([]byte(text))
	if len(document) == 0 || (document[0] != '{' && document[0] != '[') {
		return nil, &Issue{
			Message: "the string does not hold a JSON object or array",
			Hint:    "pass the value as JSON instead of a string",
		}
	}
	return document, nil
}

// syntaxIssue describes a malformed JSON document.
func syntaxIssue(err error) Issue {
	var syntaxErr *json.SyntaxError
	if errors.As(err, &syntaxErr) {
		return Issue{
			Message: fmt.Sprintf("malformed JSON at byte %d: %s", syntaxErr.Offset, syntaxErr.Error()),
			Hint:    "pass the dashboard as a JSON object rather than as an escaped string to avoid quoting mistakes",
		}
	}
	return Issue{Message: fmt.Sprintf("malformed JSON: %s", err)}
}

// decodeIssue describes a dashboard that the Perses model rejects.
func decodeIssue(err error) Issue {
	var typeErr *json.UnmarshalTypeError
	if errors.As(err, &typeErr) {
		return Issue{
			Path:    typeErr.Field,
			Message: fmt.Sprintf("expected %s, got %s", typeErr.Type, typeErr.Value),
		}
	}
	message := err.Error()
	switch {
	case strings.Contains(message, "invalid kind"):
		return Issue{Path: "kind", Message: message, Hint: `kind must be "Dashboard"`}
	case strings.Contains(message, "spec cannot be empty"):
		return Issue{Path: "spec", Message: message, Hint: "spec must contain at least panels and layouts"}
	}
	return Issue{Message: message}
}

// applyDefaults fills kind and metadata.project, and reports the metadata problems that Perses would reject.
func applyDefaults(fields map[string]any, project string) []Issue {
	var issues []Issue
	if _, ok := fields["kind"]; !ok {
		fields["kind"] = string(v1.KindDashboard)
	}

	metadata, ok := fields["metadata"].(map[string]any)
	if !ok {
		if fields["metadata"] == nil {
			metadata = map[string]any{}
			fields["metadata"] = metadata
		} else {
			return []Issue{{Path: "metadata", Message: "metadata must be an object with a name"}}
		}
	}
	if name, _ := metadata["name"].(string); name == "" {
		issues = append(issues, Issue{Path: "metadata.name", Message: "the dashboard name is required"})
	}
	switch metadataProject, _ := metadata["project"].(string); {
	case metadataProject == "":
		metadata["project"] = project
	case project != "" && !strings.EqualFold(metadataProject, project):
		issues = append(issues, Issue{
			Path:    "metadata.project",
			Message: fmt.Sprintf("metadata.project '%s' does not match project '%s'", metadataProject, project),
			Hint:    "omit metadata.project: it is filled with the project argument",
		})
	}

	if _, ok := fields["spec"].(map[string]any); !ok {
		issues = append(issues, Issue{Path: "spec", Message: "spec is required and must be an object"})
	}
	return issues
}

// checkRawReferences reports the layout items referencing a panel that does not exist. The Perses model rejects
// them with a message that does not say where the reference is.
func checkRawReferences(fields map[string]any) []Issue {
	spec, _ := fields["spec"].(map[string]any)
	panels, _ := spec["panels"].(map[string]any)
	layouts, _ := spec["layouts"].([]any)

	var issues []Issue
	for i, rawLayout := range layouts {
		layout, _ := rawLayout.(map[string]any)
		layoutSpec, _ := layout["spec"].(map[string]any)
		prefix := fmt.Sprintf("spec.layouts[%d].spec", i)
		if items, ok := layoutSpec["items"].([]any); ok {
			issues = append(issues, checkRawItems(prefix+".items", items, panels)...)
		}
		tabs, _ := layoutSpec["tabs"].([]any)
		for t, rawTab := range tabs {
			tab, _ := rawTab.(map[string]any)
			items, _ := tab["items"].([]any)
			issues = append(issues, checkRawItems(fmt.Sprintf("%s.tabs[%d].items", prefix, t), items, panels)...)
		}
	}
	return issues
}

func checkRawItems(itemsPath string, items []any, panels map[string]any) []Issue {
	var issues []Issue
	for j, rawItem := range items {
		item, _ := rawItem.(map[string]any)
		content, _ := item["content"].(map[string]any)
		ref, _ := content["$ref"].(string)
		key, isPanelRef := strings.CutPrefix(ref, panelRefPrefix)
		path := fmt.Sprintf("%s[%d].content.$ref", itemsPath, j)
		switch {
		case !isPanelRef:
			issues = append(issues, Issue{
				Path:    path,
				Message: fmt.Sprintf("invalid panel reference %q", ref),
				Hint:    "a layout item must reference a key of spec.panels as #/spec/panels/<key>",
			})
		case panels[key] == nil:
			issues = append(issues, Issue{
				Path:    path,
				Message: fmt.Sprintf("no panel found for ref %q", key),
				Hint:    "add the panel to spec.panels under this key, or remove the layout item",
			})
		}
	}
	return issues
}

// DecodeValue decodes a JSON fragment (an object or an array, possibly serialized in a string) into target. The
// path is used to locate the problem in the returned issue.
func DecodeValue(raw json.RawMessage, path string, target any) *Issue {
	document, issue := unwrap(raw)
	if issue != nil {
		issue.Path = path
		return issue
	}
	if err := json.Unmarshal(document, target); err != nil {
		issue := decodeIssue(err)
		if issue.Path == "" {
			issue.Path = path
		} else {
			issue.Path = path + "." + issue.Path
		}
		return &issue
	}
	return nil
}
