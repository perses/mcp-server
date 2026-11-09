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

package dashboard

import (
	"strings"

	dashboardSpec "github.com/perses/spec/go/dashboard"
)

// panelRefPrefix is the prefix of the JSON references of layout items to panels.
const panelRefPrefix = "#/spec/panels/"

// itemGroups returns the lists of grid items of a layout: a grid layout has one, a tabs layout has one per tab.
func itemGroups(layout dashboardSpec.Layout) [][]dashboardSpec.GridItem {
	switch spec := layout.Spec.(type) {
	case *dashboardSpec.GridLayoutSpec:
		return [][]dashboardSpec.GridItem{spec.Items}
	case *dashboardSpec.TabLayoutSpec:
		groups := make([][]dashboardSpec.GridItem, 0, len(spec.Tabs))
		for _, tab := range spec.Tabs {
			groups = append(groups, tab.Items)
		}
		return groups
	}
	return nil
}

// layoutTitle returns the title of a layout, or an empty string.
func layoutTitle(layout dashboardSpec.Layout) string {
	switch spec := layout.Spec.(type) {
	case *dashboardSpec.GridLayoutSpec:
		if spec.Display != nil {
			return spec.Display.Title
		}
	case *dashboardSpec.TabLayoutSpec:
		if spec.Display != nil {
			return spec.Display.Title
		}
	}
	return ""
}

// panelKeyOf returns the panel key referenced by a layout item, or an empty string. The parsed path of the reference
// is used when available, as the "#" prefix of the reference is optional.
func panelKeyOf(item dashboardSpec.GridItem) string {
	if item.Content == nil {
		return ""
	}
	if path := item.Content.Path; len(path) == 3 && path[0] == "spec" && path[1] == "panels" {
		return path[2]
	}
	return strings.TrimPrefix(item.Content.Ref, panelRefPrefix)
}
