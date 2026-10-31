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
	invalidNamePattern   = regexp.MustCompile(`^"([^"]*)" is not a correct name`)
	variableNamePattern  = regexp.MustCompile(`^variable name '([^']*)' (is not valid|can not have builtin variable prefix)`)
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
		issues = append(issues, issueFromBlock(block, d))
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
	return panelErrorPattern.MatchString(line) || queryErrorPattern.MatchString(line) ||
		variableErrorPattern.MatchString(line) || dtsErrorPattern.MatchString(line)
}

// issueFromBlock locates an error. The first line gives the resource and the CUE path, the other lines are details
// (e.g. the alternatives of a disjunction) whose repeated CUE path prefix is dropped.
func issueFromBlock(block []string, d *v1.Dashboard) Issue {
	head := block[0]
	switch {
	case queryErrorPattern.MatchString(head):
		match := queryErrorPattern.FindStringSubmatch(head)
		index, _ := strconv.Atoi(match[2])
		return cueIssue(fmt.Sprintf("%s.spec.queries[%d].spec.plugin", panelPath(match[1]), index-1), match[3], block[1:],
			"use perses_get_plugin_schema with the query plugin kind to see the allowed fields")
	case panelErrorPattern.MatchString(head):
		match := panelErrorPattern.FindStringSubmatch(head)
		return cueIssue(panelPath(match[1])+".spec.plugin", match[2], block[1:],
			"use perses_get_plugin_schema with the panel plugin kind to see the allowed fields")
	case variableErrorPattern.MatchString(head):
		match := variableErrorPattern.FindStringSubmatch(head)
		return cueIssue(variablePath(d, match[1])+".spec.plugin", match[2], block[1:],
			"use perses_get_plugin_schema with the variable plugin kind to see the allowed fields")
	case dtsErrorPattern.MatchString(head):
		match := dtsErrorPattern.FindStringSubmatch(head)
		return cueIssue("spec.datasources."+match[1]+".plugin", match[2], block[1:],
			"use perses_get_plugin_schema with the datasource plugin kind to see the allowed fields")
	case missingRefPattern.MatchString(head):
		key := missingRefPattern.FindStringSubmatch(head)[1]
		return Issue{Path: refPath(d, key), Message: head, Hint: "add the panel to spec.panels under this key, or remove the layout item"}
	case missingSchemaPattern.MatchString(head):
		match := missingSchemaPattern.FindStringSubmatch(head)
		return Issue{
			Path:    pluginKindPath(d, match[1]),
			Message: fmt.Sprintf("the plugin %q is not installed on the Perses server", match[1]),
			Hint:    "use perses_list_plugin_schemas to list the available plugins; plugin kinds are case-sensitive",
		}
	case invalidNamePattern.MatchString(head):
		return Issue{Path: "metadata.name", Message: head, Hint: "use letters, digits, '_', '.' and '-' only"}
	case variableNamePattern.MatchString(head):
		name := variableNamePattern.FindStringSubmatch(head)[1]
		return Issue{Path: variablePath(d, name) + ".spec.name", Message: head, Hint: "use letters, digits, '_' and '-' only, without the reserved __ prefix"}
	case strings.HasPrefix(head, "metadata.project"):
		return Issue{Path: "metadata.project", Message: head}
	}
	return Issue{Message: strings.Join(block, "; ")}
}

// cueIssue builds the issue of a CUE validation error of a plugin.
func cueIssue(pluginPath, head string, details []string, hint string) Issue {
	cuePath, message := splitCuePath(head)
	message = strings.TrimSuffix(message, ":")
	parts := []string{message}
	for _, detail := range details {
		detailPath, detailMessage := splitCuePath(detail)
		if detailPath != "" && detailPath != cuePath {
			detailMessage = detailPath + ": " + detailMessage
		}
		parts = append(parts, strings.TrimSuffix(detailMessage, ":"))
	}
	if strings.Contains(message, "field not allowed") {
		hint = "remove the field or fix its name; " + hint
	}
	return Issue{Path: pluginSpecPath(pluginPath, cuePath), Message: strings.Join(parts, "; "), Hint: hint}
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

// pluginKindPath locates the first plugin of the given kind in the dashboard.
func pluginKindPath(d *v1.Dashboard, kind string) string {
	if d == nil {
		return ""
	}
	for i, variable := range d.Spec.Variables {
		if spec, ok := variable.Spec.(*dashboard.ListVariableSpec); ok && spec.Plugin.Kind == kind {
			return fmt.Sprintf("spec.variables[%d].spec.plugin.kind", i)
		}
	}
	for _, key := range sortedPanelKeys(d) {
		panel := d.Spec.Panels[key]
		if panel.Spec.Plugin.Kind == kind {
			return panelPath(key) + ".spec.plugin.kind"
		}
		for j, query := range panel.Spec.Queries {
			if query.Spec.Plugin.Kind == kind {
				return fmt.Sprintf("%s.spec.queries[%d].spec.plugin.kind", panelPath(key), j)
			}
		}
	}
	for name, dts := range d.Spec.Datasources {
		if dts != nil && dts.Plugin.Kind == kind {
			return "spec.datasources." + name + ".plugin.kind"
		}
	}
	return ""
}
