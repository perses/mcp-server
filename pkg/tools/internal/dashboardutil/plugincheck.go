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
	"fmt"
	"slices"
	"strings"
)

// queryPluginExamples maps the query kinds to an example of their plugin kind.
var queryPluginExamples = map[string]string{
	"TimeSeriesQuery": "PrometheusTimeSeriesQuery",
	"TraceQuery":      "TempoTraceQuery",
	"LogQuery":        "LokiLogQuery",
	"ProfileQuery":    "PyroscopeProfileQuery",
}

// checkRawPanels reports the panels and queries without a plugin kind. The Perses model rejects them with
// "kind cannot be empty", which does not say where the plugin is missing nor how the value must be wrapped.
func checkRawPanels(panelsPath string, panels map[string]any) []Issue {
	keys := make([]string, 0, len(panels))
	for key := range panels {
		keys = append(keys, key)
	}
	slices.Sort(keys)

	var issues []Issue
	for _, key := range keys {
		panel, _ := panels[key].(map[string]any)
		panelSpec, _ := panel["spec"].(map[string]any)
		if panelSpec == nil {
			continue
		}
		path := keyPath(panelsPath, key) + ".spec"
		if pluginKind(panelSpec) == "" {
			issues = append(issues, Issue{
				Path:    path + ".plugin",
				Message: "panel plugin is missing",
				Hint:    `panels need a plugin: {kind: "TimeSeriesChart", spec: {...}}; perses_list_plugins lists the panel plugins`,
			})
		}
		queries, _ := panelSpec["queries"].([]any)
		for i, rawQuery := range queries {
			if query, ok := rawQuery.(map[string]any); ok {
				if issue := checkRawQuery(fmt.Sprintf("%s.queries[%d]", path, i), query); issue != nil {
					issues = append(issues, *issue)
				}
			}
		}
	}
	return issues
}

func checkRawQuery(path string, query map[string]any) *Issue {
	querySpec, _ := query["spec"].(map[string]any)
	if pluginKind(querySpec) != "" {
		return nil
	}
	kind, _ := query["kind"].(string)
	if example, isQueryKind := queryPluginExamples[kind]; isQueryKind || kind == "" {
		if kind == "" {
			kind, example = "TimeSeriesQuery", queryPluginExamples["TimeSeriesQuery"]
		}
		return &Issue{
			Path:    path,
			Message: "query plugin is missing",
			Hint:    fmt.Sprintf(`queries need a wrapper: {kind: "%s", spec: {plugin: {kind: "%s", spec: {...}}}}`, kind, example),
		}
	}
	// The plugin was given in place of the query: show the wrapper around it.
	return &Issue{
		Path:    path,
		Message: fmt.Sprintf("query plugin is missing: %q is not a query kind", kind),
		Hint: fmt.Sprintf(`queries need a wrapper: {kind: "%s", spec: {plugin: {kind: "%s", spec: {%s}}}}`,
			queryKindFor(kind), kind, fieldsSketch(querySpec)),
	}
}

// checkRawVariables reports the list variables without a plugin kind.
func checkRawVariables(variablesPath string, variables []any) []Issue {
	var issues []Issue
	for i, rawVariable := range variables {
		variable, _ := rawVariable.(map[string]any)
		variableSpec, _ := variable["spec"].(map[string]any)
		if variable["kind"] != "ListVariable" || variableSpec == nil || pluginKind(variableSpec) != "" {
			continue
		}
		issues = append(issues, Issue{
			Path:    fmt.Sprintf("%s[%d].spec.plugin", variablesPath, i),
			Message: "list variable plugin is missing",
			Hint:    `list variables need a plugin: {kind: "PrometheusLabelValuesVariable", spec: {...}}`,
		})
	}
	return issues
}

// pluginKind returns the kind of the plugin of a raw spec, empty when there is none.
func pluginKind(spec map[string]any) string {
	plugin, _ := spec["plugin"].(map[string]any)
	kind, _ := plugin["kind"].(string)
	return kind
}

// queryKindFor guesses the query kind of a query plugin kind, e.g. TraceQuery for TempoTraceQuery.
func queryKindFor(pluginKind string) string {
	for _, queryKind := range []string{"TraceQuery", "LogQuery", "ProfileQuery"} {
		if strings.HasSuffix(pluginKind, queryKind) {
			return queryKind
		}
	}
	return "TimeSeriesQuery"
}

// fieldsSketch lists the fields of a raw spec as "a: ..., b: ...", or "..." when it has none.
func fieldsSketch(spec map[string]any) string {
	if len(spec) == 0 {
		return "..."
	}
	keys := make([]string, 0, len(spec))
	for key := range spec {
		keys = append(keys, key+": ...")
	}
	slices.Sort(keys)
	return strings.Join(keys, ", ")
}
