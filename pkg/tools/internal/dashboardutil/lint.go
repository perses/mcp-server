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
	"sort"
	"strings"

	v1 "github.com/perses/perses/pkg/model/api/v1"
	"github.com/perses/spec/go/dashboard"
)

// GridColumns is the number of columns of a Perses grid layout.
const GridColumns = 24

// Lint runs the local checks that the Perses server does not perform, or performs with a message that does not say
// where the problem is. Errors are mistakes that break the dashboard rendering; warnings are accepted by Perses but
// probably unintended.
func Lint(d *v1.Dashboard) (errs, warnings []Issue) {
	referenced := map[string]int{}
	for i, layout := range d.Spec.Layouts {
		for _, group := range itemGroups(i, layout) {
			for j, item := range group.items {
				if key := panelKeyOf(item); key != "" {
					referenced[key]++
				}
				errs = append(errs, lintGridItem(group.path, j, item)...)
			}
			warnings = append(warnings, lintOverlaps(group.path, group.items)...)
		}
	}

	keys := make([]string, 0, len(d.Spec.Panels))
	for key := range d.Spec.Panels {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var orphans []string
	for _, key := range keys {
		if referenced[key] == 0 {
			orphans = append(orphans, key)
		}
	}
	switch len(orphans) {
	case 0:
	case 1:
		warnings = append(warnings, Issue{
			Path:    panelPath(orphans[0]),
			Message: "the panel is not displayed: no layout item references it",
			Hint:    fmt.Sprintf("add an item with content {\"$ref\": \"%s%s\"} to a Grid layout, or remove the panel", panelRefPrefix, orphans[0]),
		})
	default:
		// One warning for all orphans keeps the output short when a whole group was emptied.
		warnings = append(warnings, Issue{
			Path:    "spec.panels",
			Message: fmt.Sprintf("%d panels are not displayed: no layout item references %s", len(orphans), strings.Join(orphans, ", ")),
			Hint:    fmt.Sprintf("add items with content {\"$ref\": \"%s<key>\"} to a Grid layout (perses_set_dashboard_panels places them automatically), or remove the panels", panelRefPrefix),
		})
	}

	seen := map[string]int{}
	for i, variable := range d.Spec.Variables {
		if variable.Spec == nil {
			continue
		}
		name := variable.Spec.GetName()
		if first, ok := seen[name]; ok {
			errs = append(errs, Issue{
				Path:    fmt.Sprintf("spec.variables[%d].spec.name", i),
				Message: fmt.Sprintf("the variable name %q is already used by spec.variables[%d]", name, first),
				Hint:    "variable names must be unique in a dashboard",
			})
			continue
		}
		seen[name] = i
	}
	return errs, warnings
}

func lintGridItem(itemsPath string, index int, item dashboard.GridItem) []Issue {
	var issues []Issue
	path := fmt.Sprintf("%s[%d]", itemsPath, index)
	if item.Width < 1 || item.Width > GridColumns {
		issues = append(issues, Issue{
			Path:    path + ".width",
			Message: fmt.Sprintf("the width %d is out of range", item.Width),
			Hint:    fmt.Sprintf("the grid has %d columns: use a width between 1 and %d", GridColumns, GridColumns),
		})
	} else if item.X < 0 || item.X+item.Width > GridColumns {
		issues = append(issues, Issue{
			Path:    path + ".x",
			Message: fmt.Sprintf("the item spans columns %d to %d, beyond the %d columns of the grid", item.X, item.X+item.Width, GridColumns),
			Hint:    fmt.Sprintf("x + width must not exceed %d", GridColumns),
		})
	}
	if item.Height < 1 {
		issues = append(issues, Issue{
			Path:    path + ".height",
			Message: fmt.Sprintf("the height %d is out of range", item.Height),
			Hint:    "use a height of at least 1 row (a row is 30px; charts usually use 6 to 8)",
		})
	}
	if item.Y < 0 {
		issues = append(issues, Issue{Path: path + ".y", Message: fmt.Sprintf("the y position %d is negative", item.Y)})
	}
	return issues
}

// lintOverlaps warns about grid items that overlap: the UI pushes them down, which changes the intended layout.
func lintOverlaps(itemsPath string, items []dashboard.GridItem) []Issue {
	var issues []Issue
	for i := range items {
		for j := i + 1; j < len(items); j++ {
			if overlaps(items[i], items[j]) {
				issues = append(issues, Issue{
					Path:    fmt.Sprintf("%s[%d]", itemsPath, j),
					Message: fmt.Sprintf("the item overlaps %s[%d] and will be pushed down by the UI", itemsPath, i),
					Hint:    "give each item of a row a distinct x range, and each row a distinct y",
				})
			}
		}
	}
	return issues
}

func overlaps(a, b dashboard.GridItem) bool {
	return a.X < b.X+b.Width && b.X < a.X+a.Width && a.Y < b.Y+b.Height && b.Y < a.Y+a.Height
}
