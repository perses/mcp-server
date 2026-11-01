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
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/perses/perses/pkg/client/perseshttp"
	v1 "github.com/perses/perses/pkg/model/api/v1"
	"github.com/perses/spec/go/dashboard"
	"github.com/perses/spec/go/plugin"
)

// The Perses server reports CUE validation errors as text. These patterns match the start of each error, as written
// by perses/internal/api/plugin/schema and perses/internal/api/validate. The CUE path that follows is relative to the
// plugin ({kind, spec}), and the error is followed by indented lines giving positions in the CUE schema files.
var (
	panelErrorPattern    = regexp.MustCompile(`^invalid panel (\S+): (.*)$`)
	queryErrorPattern    = regexp.MustCompile(`^panel "([^"]+)": invalid query n°(\d+): (.*)$`)
	variableErrorPattern = regexp.MustCompile(`^invalid variable (\S+): (.*)$`)
	dtsErrorPattern      = regexp.MustCompile(`^invalid datasource (\S+): (.*)$`)
	// cuePathPattern matches the CUE path prefix of a validation message, e.g. "spec.legend.position: ".
	cuePathPattern = regexp.MustCompile(`^((?:kind|spec)(?:\.[A-Za-z0-9_$-]+|\[\d+])*): (.*)$`)
	// echoErrorPattern matches the echo HTTPError wrapping, e.g. `code=400, message=foo, internal=foo`.
	echoErrorPattern     = regexp.MustCompile(`^code=\d+, message=(.*), internal=(.*)$`)
	missingRefPattern    = regexp.MustCompile(`^no panel found for ref "([^"]+)"$`)
	missingSchemaPattern = regexp.MustCompile(`^schema not found for plugin (\S+) with version (\S+)$`)
	// panelPrefixPattern matches the errors of the queries and annotations of a panel, e.g. `panel "cpu": ...`.
	panelPrefixPattern = regexp.MustCompile(`^panel "([^"]+)": (.*)$`)
	// annotationErrorPattern matches the CUE validation error of an annotation, e.g. "invalid annotation Deploys: ...".
	// The name is the display name of the annotation, which may contain spaces and colons, or n°<index> for an unnamed
	// panel annotation.
	annotationErrorPattern = regexp.MustCompile(`^invalid annotation (.*)$`)
	// annotationFallbackPattern splits the name of the annotation from the CUE error when the dashboard is unknown.
	annotationFallbackPattern = regexp.MustCompile(`^(.*?): ((?:kind|spec)(?:\.[A-Za-z0-9_$-]+|\[\d+])*: .*)$`)
	invalidNamePattern        = regexp.MustCompile(`^"([^"]*)" is not a correct name`)
	variableNamePattern       = regexp.MustCompile(`^variable name '([^']*)' (is not valid|can not have builtin variable prefix)`)
)

// IsBadRequest reports whether the error is a 400 response of the Perses API, i.e. a validation failure.
func IsBadRequest(err error) (string, bool) {
	var requestErr *perseshttp.RequestError
	if !errors.As(err, &requestErr) || requestErr.StatusCode != http.StatusBadRequest {
		return "", false
	}
	message := requestErr.Message
	if message == "" {
		message = err.Error()
	}
	return message, true
}

// IsConflict reports whether the error is a 409 response of the Perses API: the resource already exists.
func IsConflict(err error) bool {
	var requestErr *perseshttp.RequestError
	return errors.As(err, &requestErr) && requestErr.StatusCode == http.StatusConflict
}

// IsNotFound reports whether the error is a 404 response of the Perses API.
func IsNotFound(err error) bool {
	var requestErr *perseshttp.RequestError
	return errors.As(err, &requestErr) && requestErr.StatusCode == http.StatusNotFound
}

// IssuesFromServerMessage converts the validation message of the Perses API into located issues. The dashboard,
// when given, is used to resolve the positions that the message only gives by name (e.g. a variable index).
func IssuesFromServerMessage(message string, d *v1.Dashboard) []Issue {
	message = cleanServerMessage(message)
	var issues []Issue
	for _, block := range splitErrorBlocks(message) {
		issues = append(issues, issuesFromBlock(block, d)...)
	}
	if len(issues) == 0 {
		issues = append(issues, Issue{Message: message})
	}
	return issues
}

// resourcePluginErrorPattern matches the CUE validation error of a standalone datasource or variable. The name is
// empty for variables, e.g. "invalid variable : spec.foo: field not allowed".
var resourcePluginErrorPattern = regexp.MustCompile(`^invalid (?:datasource|variable) (?:\S+)?: (.*)$`)

// ResourceIssuesFromServerMessage converts the validation message of the Perses API for a standalone datasource or
// variable into located issues. pluginPath is the JSON path of its plugin, e.g. spec.plugin for a datasource.
func ResourceIssuesFromServerMessage(message, pluginPath, hint string) []Issue {
	message = cleanServerMessage(message)
	var issues []Issue
	for _, block := range splitErrorBlocks(message) {
		head := block[0]
		switch {
		case resourcePluginErrorPattern.MatchString(head):
			issues = append(issues, cueIssues(pluginPath, resourcePluginErrorPattern.FindStringSubmatch(head)[1], block[1:], hint)...)
		case missingSchemaPattern.MatchString(head):
			match := missingSchemaPattern.FindStringSubmatch(head)
			issues = append(issues, missingSchemaIssue(pluginPath, match[1], match[2]))
		case invalidNamePattern.MatchString(head):
			issues = append(issues, Issue{Path: "metadata.name", Message: head, Hint: "use letters, digits, '_', '.' and '-' only"})
		case variableNamePattern.MatchString(head):
			issues = append(issues, Issue{Path: "metadata.name", Message: head, Hint: "use letters, digits, '_' and '-' only, without the reserved __ prefix"})
		default:
			issues = append(issues, Issue{Message: strings.Join(block, "; ")})
		}
	}
	if len(issues) == 0 {
		issues = append(issues, Issue{Message: message})
	}
	return issues
}

// cleanServerMessage removes the wrappings that the Perses API adds around the validation message.
func cleanServerMessage(message string) string {
	message = strings.TrimSpace(message)
	for {
		trimmed := strings.TrimPrefix(message, "bad request: ")
		if trimmed == message {
			break
		}
		message = trimmed
	}
	if match := echoErrorPattern.FindStringSubmatch(message); match != nil {
		message = match[1]
	}
	return message
}

// splitErrorBlocks splits a message into one block per error. Perses joins the errors of all the panels with new
// lines, and each CUE error is followed by indented position lines that carry no information for the caller.
func splitErrorBlocks(message string) [][]string {
	var blocks [][]string
	for _, line := range strings.Split(message, "\n") {
		if strings.TrimSpace(line) == "" || strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
			continue
		}
		if len(blocks) == 0 || startsError(line) {
			blocks = append(blocks, []string{line})
			continue
		}
		blocks[len(blocks)-1] = append(blocks[len(blocks)-1], line)
	}
	return blocks
}

func startsError(line string) bool {
	return panelErrorPattern.MatchString(line) || panelPrefixPattern.MatchString(line) || annotationErrorPattern.MatchString(line) ||
		variableErrorPattern.MatchString(line) || dtsErrorPattern.MatchString(line) || missingSchemaPattern.MatchString(line)
}

// issuesFromBlock locates an error. The first line gives the resource and the CUE path, the other lines are details
// (e.g. the alternatives of a disjunction, or the errors of other fields of the same plugin).
func issuesFromBlock(block []string, d *v1.Dashboard) []Issue {
	head := block[0]
	if match := panelPrefixPattern.FindStringSubmatch(head); match != nil && annotationErrorPattern.MatchString(match[2]) {
		var annotations []dashboard.AnnotationSpec
		if d != nil && d.Spec.Panels[match[1]] != nil {
			annotations = d.Spec.Panels[match[1]].Spec.Annotations
		}
		rest := annotationErrorPattern.FindStringSubmatch(match[2])[1]
		return annotationIssues(panelPath(match[1])+".spec.annotations", annotations, true, rest, block[1:])
	}
	if match := annotationErrorPattern.FindStringSubmatch(head); match != nil {
		var annotations []dashboard.AnnotationSpec
		if d != nil {
			annotations = d.Spec.Annotations
		}
		return annotationIssues("spec.annotations", annotations, false, match[1], block[1:])
	}
	// The missing schema of a query or an annotation of a panel is reported as
	// `panel "<key>": schema not found for plugin ...`.
	if match := panelPrefixPattern.FindStringSubmatch(head); match != nil && missingSchemaPattern.MatchString(match[2]) {
		schema := missingSchemaPattern.FindStringSubmatch(match[2])
		return []Issue{missingSchemaIssue(panelPluginPath(d, match[1], schema[1], schema[2]), schema[1], schema[2])}
	}
	switch {
	case queryErrorPattern.MatchString(head):
		match := queryErrorPattern.FindStringSubmatch(head)
		index, _ := strconv.Atoi(match[2])
		return cueIssues(fmt.Sprintf("%s.spec.queries[%d].spec.plugin", panelPath(match[1]), index-1), match[3], block[1:],
			"use perses_get_plugin_schema with the query plugin kind to see the allowed fields")
	case panelErrorPattern.MatchString(head):
		match := panelErrorPattern.FindStringSubmatch(head)
		return cueIssues(panelPath(match[1])+".spec.plugin", match[2], block[1:],
			"use perses_get_plugin_schema with the panel plugin kind to see the allowed fields")
	case variableErrorPattern.MatchString(head):
		match := variableErrorPattern.FindStringSubmatch(head)
		return cueIssues(variablePath(d, match[1])+".spec.plugin", match[2], block[1:],
			"use perses_get_plugin_schema with the variable plugin kind to see the allowed fields")
	case dtsErrorPattern.MatchString(head):
		match := dtsErrorPattern.FindStringSubmatch(head)
		return cueIssues(keyPath("spec.datasources", match[1])+".plugin", match[2], block[1:],
			"use perses_get_plugin_schema with the datasource plugin kind to see the allowed fields")
	case missingRefPattern.MatchString(head):
		key := missingRefPattern.FindStringSubmatch(head)[1]
		return []Issue{{Path: refPath(d, key), Message: head, Hint: "add the panel to spec.panels under this key, or remove the layout item"}}
	case missingSchemaPattern.MatchString(head):
		match := missingSchemaPattern.FindStringSubmatch(head)
		return []Issue{missingSchemaIssue(pluginPathOf(d, match[1], match[2]), match[1], match[2])}
	case invalidNamePattern.MatchString(head):
		return []Issue{{Path: "metadata.name", Message: head, Hint: "use letters, digits, '_', '.' and '-' only"}}
	case variableNamePattern.MatchString(head):
		name := variableNamePattern.FindStringSubmatch(head)[1]
		return []Issue{{Path: variablePath(d, name) + ".spec.name", Message: head, Hint: "use letters, digits, '_' and '-' only, without the reserved __ prefix"}}
	case strings.HasPrefix(head, "metadata.project"):
		return []Issue{{Path: "metadata.project", Message: head}}
	}
	return []Issue{{Message: strings.Join(block, "; ")}}
}

// annotationIssues locates the CUE validation errors of an annotation. rest is the error after "invalid annotation ":
// the name of the annotation, then the CUE error. Perses names the unnamed panel annotations n°<index> (1-based).
func annotationIssues(parent string, annotations []dashboard.AnnotationSpec, inPanel bool, rest string, details []string) []Issue {
	const hint = "use perses_get_plugin_schema with the annotation plugin kind to see the allowed fields"
	index, nameLength, candidates := -1, -1, 0
	for i, annotation := range annotations {
		name := annotation.Display.Name
		if name == "" && inPanel {
			name = fmt.Sprintf("n°%d", i+1)
		}
		if !strings.HasPrefix(rest, name+": ") {
			continue
		}
		// The longest name wins, in case a name is a prefix of another one.
		switch {
		case len(name) > nameLength:
			index, nameLength, candidates = i, len(name), 1
		case len(name) == nameLength:
			candidates++
		}
	}
	// The error does not tell apart the annotations that have the same name: they are located by name.
	if candidates == 1 {
		return cueIssues(fmt.Sprintf("%s[%d].plugin", parent, index), rest[nameLength+2:], details, hint)
	}
	name, cueError, found := strings.Cut(rest, ": ")
	if match := annotationFallbackPattern.FindStringSubmatch(rest); match != nil {
		name, cueError = match[1], match[2]
	} else if !found {
		return []Issue{{Path: parent, Message: strings.Join(append([]string{rest}, details...), "; ")}}
	}
	return cueIssues(fmt.Sprintf("%s[name=%s].plugin", parent, name), cueError, details, hint)
}

// cueIssues builds the issues of the CUE validation errors of a plugin. Perses reports every error of the plugin in
// one message: each error at another CUE path is a separate issue, while the alternatives of an empty disjunction
// stay with it.
func cueIssues(pluginPath, head string, details []string, hint string) []Issue {
	type cueError struct {
		path  string
		parts []string
	}
	cuePath, message := splitCuePath(head)
	errs := []cueError{{path: cuePath, parts: []string{strings.TrimSuffix(message, ":")}}}
	for _, detail := range details {
		detailPath, detailMessage := splitCuePath(detail)
		current := &errs[len(errs)-1]
		if detailPath != "" && detailPath != current.path {
			if !strings.Contains(current.parts[0], "errors in empty disjunction") || !isWithin(detailPath, current.path) {
				errs = append(errs, cueError{path: detailPath, parts: []string{strings.TrimSuffix(detailMessage, ":")}})
				continue
			}
			detailMessage = detailPath + ": " + detailMessage
		}
		current.parts = append(current.parts, strings.TrimSuffix(detailMessage, ":"))
	}
	issues := make([]Issue, 0, len(errs))
	for _, cueErr := range errs {
		issueHint := hint
		if strings.Contains(cueErr.parts[0], "field not allowed") {
			issueHint = "remove the field or fix its name; " + hint
		}
		issues = append(issues, Issue{Path: pluginSpecPath(pluginPath, cueErr.path), Message: strings.Join(cueErr.parts, "; "), Hint: issueHint})
	}
	return issues
}

// isWithin reports whether the CUE path is the parent path or one of its descendants.
func isWithin(path, parent string) bool {
	return parent == "" || path == parent || strings.HasPrefix(path, parent+".") || strings.HasPrefix(path, parent+"[")
}

func splitCuePath(line string) (string, string) {
	if match := cuePathPattern.FindStringSubmatch(line); match != nil {
		return match[1], match[2]
	}
	return "", line
}

// variablePath locates a variable by name, falling back to a name selector when the dashboard is unknown.
func variablePath(d *v1.Dashboard, name string) string {
	if d != nil {
		for i, variable := range d.Spec.Variables {
			if variable.Spec != nil && variable.Spec.GetName() == name {
				return fmt.Sprintf("spec.variables[%d]", i)
			}
		}
	}
	return fmt.Sprintf("spec.variables[name=%s]", name)
}

// refPath locates the first layout item referencing a panel key.
func refPath(d *v1.Dashboard, key string) string {
	if d == nil {
		return ""
	}
	for i, layout := range d.Spec.Layouts {
		for _, group := range itemGroups(i, layout) {
			for j, item := range group.items {
				if panelKeyOf(item) == key {
					return fmt.Sprintf("%s[%d].content.$ref", group.path, j)
				}
			}
		}
	}
	return ""
}

// missingSchemaIssue reports a plugin whose schema Perses does not have. Perses looks the schema up by the kind and
// the metadata (version, registry) of the plugin, so the kind may be right and the requested version or registry
// unavailable. Perses writes the version "unknown" when the plugin does not set one.
func missingSchemaIssue(path, kind, version string) Issue {
	requested := fmt.Sprintf("%q", kind)
	if version != "unknown" {
		requested = fmt.Sprintf("%q with version %s", kind, version)
	}
	return Issue{
		Path: path,
		Message: fmt.Sprintf("the Perses server has no schema for the plugin %s: the plugin is not installed, "+
			"or not with the requested version or registry", requested),
		Hint: "check the kind of the plugin (case-sensitive) and its metadata (version, registry); " +
			"use perses_list_plugin_schemas to list the available plugins",
	}
}

// matchesPlugin reports whether the plugin is the one that Perses reports with this kind and version.
func matchesPlugin(p plugin.Plugin, kind, version string) bool {
	if p.Kind != kind {
		return false
	}
	if p.Metadata == nil || p.Metadata.Version == "" {
		return version == "unknown"
	}
	return p.Metadata.Version == version
}

// panelPluginPath locates the first query or annotation of the panel matching the plugin that Perses reports with
// the panel prefix.
func panelPluginPath(d *v1.Dashboard, key, kind, version string) string {
	if d == nil || d.Spec.Panels[key] == nil {
		return ""
	}
	panel := d.Spec.Panels[key]
	for j, query := range panel.Spec.Queries {
		if matchesPlugin(query.Spec.Plugin, kind, version) {
			return fmt.Sprintf("%s.spec.queries[%d].spec.plugin", panelPath(key), j)
		}
	}
	for j, annotation := range panel.Spec.Annotations {
		if matchesPlugin(annotation.Plugin, kind, version) {
			return fmt.Sprintf("%s.spec.annotations[%d].plugin", panelPath(key), j)
		}
	}
	return ""
}

// pluginPathOf locates the first plugin matching the plugin that Perses reports without a panel prefix: a variable,
// a panel, a dashboard annotation or a datasource. The queries and annotations of the panels have the prefix.
func pluginPathOf(d *v1.Dashboard, kind, version string) string {
	if d == nil {
		return ""
	}
	for i, variable := range d.Spec.Variables {
		if spec, ok := variable.Spec.(*dashboard.ListVariableSpec); ok && matchesPlugin(spec.Plugin, kind, version) {
			return fmt.Sprintf("spec.variables[%d].spec.plugin", i)
		}
	}
	for _, key := range sortedPanelKeys(d) {
		if matchesPlugin(d.Spec.Panels[key].Spec.Plugin, kind, version) {
			return panelPath(key) + ".spec.plugin"
		}
	}
	for i, annotation := range d.Spec.Annotations {
		if matchesPlugin(annotation.Plugin, kind, version) {
			return fmt.Sprintf("spec.annotations[%d].plugin", i)
		}
	}
	for name, dts := range d.Spec.Datasources {
		if dts != nil && matchesPlugin(dts.Plugin, kind, version) {
			return keyPath("spec.datasources", name) + ".plugin"
		}
	}
	return ""
}
