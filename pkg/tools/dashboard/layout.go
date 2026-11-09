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
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/perses/spec/go/common"
	dashboardSpec "github.com/perses/spec/go/dashboard"
)

// Grid layouts are panel groups as built by the Perses SDKs: every panel of a group has the same size.
const (
	gridColumns = 24
	// Default panel size of a group, as in go-sdk/panel-group: 2 panels per line, 8 rows high.
	defaultPanelsPerLine = 2
	defaultPanelHeight   = 8
	panelRefPrefix       = "#/spec/panels/"
)

// gridItems lays out the panels of a group in order, panelsPerLine per row, like go-sdk/dashboard.AddPanelGroup and
// cue/dac-utils/panelgroup of github.com/perses/perses. It follows the CUE rule, which never overflows the grid
// (https://github.com/perses/perses/issues/4716). The formula is local because the SDK builds a new dashboard with
// its own panel keys and exports no placement function (https://github.com/perses/perses/issues/4717).
func gridItems(keys []string, panelsPerLine, height int) []dashboardSpec.GridItem {
	width := gridColumns / panelsPerLine
	items := make([]dashboardSpec.GridItem, 0, len(keys))
	for i, key := range keys {
		items = append(items, dashboardSpec.GridItem{
			X:       width * (i % panelsPerLine),
			Y:       height * (i / panelsPerLine),
			Width:   width,
			Height:  height,
			Content: &common.JSONRef{Ref: panelRefPrefix + key, Path: []string{"spec", "panels", key}},
		})
	}
	return items
}

// groupPanels returns the panel keys of a group in reading order (top to bottom, then left to right), and the
// panels per line and panel height of the group, inferred from its first item, or the SDK defaults.
func groupPanels(grid *dashboardSpec.GridLayoutSpec) ([]string, int, int) {
	items := slices.Clone(grid.Items)
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].Y < items[j].Y || (items[i].Y == items[j].Y && items[i].X < items[j].X)
	})
	keys := make([]string, 0, len(items))
	for _, item := range items {
		if key := panelKeyOf(item); key != "" {
			keys = append(keys, key)
		}
	}
	if len(items) == 0 || items[0].Width <= 0 || items[0].Height <= 0 {
		return keys, defaultPanelsPerLine, defaultPanelHeight
	}
	return keys, max(gridColumns/items[0].Width, 1), items[0].Height
}

// keepRepeats copies the repeat settings of the panels from their current Grid items to the rebuilt items.
func keepRepeats(items []dashboardSpec.GridItem, layouts []dashboardSpec.Layout) {
	repeats := map[string]*dashboardSpec.RepeatVariable{}
	for _, layout := range layouts {
		if grid, ok := layout.Spec.(*dashboardSpec.GridLayoutSpec); ok {
			for _, item := range grid.Items {
				if item.RepeatVariable != nil {
					repeats[panelKeyOf(item)] = item.RepeatVariable
				}
			}
		}
	}
	for i := range items {
		items[i].RepeatVariable = repeats[panelKeyOf(items[i])]
	}
}

// findGroup returns the index of the Grid layout with the given title, or -1. An exact title match is preferred;
// otherwise the case is ignored, and several groups matching that way are an error. An empty title designates the
// last Grid layout.
func findGroup(layouts []dashboardSpec.Layout, title string) (int, error) {
	var folded []int
	for i := len(layouts) - 1; i >= 0; i-- {
		if _, ok := layouts[i].Spec.(*dashboardSpec.GridLayoutSpec); !ok {
			continue
		}
		current := layoutTitle(layouts[i])
		if title == "" || current == title {
			return i, nil
		}
		if strings.EqualFold(current, title) {
			folded = append(folded, i)
		}
	}
	switch len(folded) {
	case 0:
		return -1, nil
	case 1:
		return folded[0], nil
	}
	titles := make([]string, 0, len(folded))
	for _, i := range slices.Backward(folded) {
		titles = append(titles, fmt.Sprintf("%q", layoutTitle(layouts[i])))
	}
	return -1, fmt.Errorf("no Grid group is titled exactly %q and several match ignoring case (%s): give the exact title", title, strings.Join(titles, ", "))
}

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

// isPlaced reports whether a layout item references the panel key.
func isPlaced(layouts []dashboardSpec.Layout, key string) bool {
	for _, layout := range layouts {
		for _, items := range itemGroups(layout) {
			for _, item := range items {
				if panelKeyOf(item) == key {
					return true
				}
			}
		}
	}
	return false
}

// removePanelItems removes the layout items referencing the panel keys, from Grid layouts only unless tabs is true,
// and returns the number of items removed.
func removePanelItems(layouts []dashboardSpec.Layout, keys map[string]bool, tabs bool) int {
	removed := 0
	filter := func(items []dashboardSpec.GridItem) []dashboardSpec.GridItem {
		kept := items[:0]
		for _, item := range items {
			if keys[panelKeyOf(item)] {
				removed++
				continue
			}
			kept = append(kept, item)
		}
		return kept
	}
	for _, layout := range layouts {
		switch spec := layout.Spec.(type) {
		case *dashboardSpec.GridLayoutSpec:
			spec.Items = filter(spec.Items)
		case *dashboardSpec.TabLayoutSpec:
			if tabs {
				for i := range spec.Tabs {
					spec.Tabs[i].Items = filter(spec.Tabs[i].Items)
				}
			}
		}
	}
	return removed
}
