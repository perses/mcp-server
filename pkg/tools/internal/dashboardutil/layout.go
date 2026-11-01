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
	"strings"

	"github.com/perses/spec/go/common"
	"github.com/perses/spec/go/dashboard"
)

// GridColumns is the number of columns of a Perses grid layout.
const GridColumns = 24

// Default size of a panel placed automatically: two panels per row, 6 rows (180px) high.
const (
	DefaultPanelWidth  = 12
	DefaultPanelHeight = 6
)

// panelRefPrefix is the prefix of the JSON references of layout items to panels.
const panelRefPrefix = "#/spec/panels/"

// itemGroups returns the lists of grid items of a layout: a grid layout has one, a tabs layout has one per tab.
func itemGroups(layout dashboard.Layout) [][]dashboard.GridItem {
	switch spec := layout.Spec.(type) {
	case *dashboard.GridLayoutSpec:
		return [][]dashboard.GridItem{spec.Items}
	case *dashboard.TabLayoutSpec:
		groups := make([][]dashboard.GridItem, 0, len(spec.Tabs))
		for _, tab := range spec.Tabs {
			groups = append(groups, tab.Items)
		}
		return groups
	}
	return nil
}

// layoutTitle returns the title of a layout, or an empty string.
func layoutTitle(layout dashboard.Layout) string {
	switch spec := layout.Spec.(type) {
	case *dashboard.GridLayoutSpec:
		if spec.Display != nil {
			return spec.Display.Title
		}
	case *dashboard.TabLayoutSpec:
		if spec.Display != nil {
			return spec.Display.Title
		}
	}
	return ""
}

// panelKeyOf returns the panel key referenced by a layout item, or an empty string.
func panelKeyOf(item dashboard.GridItem) string {
	if item.Content == nil {
		return ""
	}
	return strings.TrimPrefix(item.Content.Ref, panelRefPrefix)
}

// PanelRef builds the JSON reference of a panel key, as used by layout items.
func PanelRef(key string) *common.JSONRef {
	return &common.JSONRef{Ref: panelRefPrefix + key, Path: []string{"spec", "panels", key}}
}

// LayoutTitle returns the title of a layout, or an empty string.
func LayoutTitle(layout dashboard.Layout) string {
	return layoutTitle(layout)
}

// FindLayoutByTitle returns the index of the layout with the given title (ignoring case), or -1.
func FindLayoutByTitle(layouts []dashboard.Layout, title string) int {
	for i, layout := range layouts {
		if strings.EqualFold(layoutTitle(layout), title) {
			return i
		}
	}
	return -1
}

// IsPlaced reports whether a layout item references the panel key.
func IsPlaced(layouts []dashboard.Layout, key string) bool {
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

// FindLayout returns the index of the grid layout with the given title (ignoring case), or -1. An empty title
// designates the last grid layout, so that panels are appended at the bottom of the dashboard by default.
func FindLayout(layouts []dashboard.Layout, title string) int {
	for i := len(layouts) - 1; i >= 0; i-- {
		if _, ok := layouts[i].Spec.(*dashboard.GridLayoutSpec); !ok {
			continue
		}
		if title == "" || strings.EqualFold(layoutTitle(layouts[i]), title) {
			return i
		}
	}
	return -1
}

// EnsureLayout returns the index of the grid layout with the given title, appending a new one when it does not
// exist. An empty title designates the last grid layout, created without a title when the dashboard has none.
func EnsureLayout(layouts *[]dashboard.Layout, title string) int {
	if index := FindLayout(*layouts, title); index >= 0 {
		return index
	}
	spec := &dashboard.GridLayoutSpec{Items: []dashboard.GridItem{}}
	if title != "" {
		spec.Display = &dashboard.GridLayoutDisplay{Title: title, Collapse: &dashboard.GridLayoutCollapse{Open: true}}
	}
	*layouts = append(*layouts, dashboard.Layout{Kind: dashboard.KindGridLayout, Spec: spec})
	return len(*layouts) - 1
}

// PlacePanel adds an item referencing the panel key to the grid layout, at the lowest free position that fits the
// width, leftmost first. Panels of width 12 are thus laid out two per row, under the existing items. The width is
// clamped to the grid.
func PlacePanel(layout *dashboard.Layout, key string, width, height int) (dashboard.GridItem, error) {
	grid, ok := layout.Spec.(*dashboard.GridLayoutSpec)
	if !ok {
		return dashboard.GridItem{}, fmt.Errorf("layout of kind %q is not a grid", layout.Kind)
	}
	if width <= 0 {
		width = DefaultPanelWidth
	}
	width = min(width, GridColumns)
	if height <= 0 {
		height = DefaultPanelHeight
	}

	// skyline[c] is the first free row of column c.
	var skyline [GridColumns]int
	for _, item := range grid.Items {
		for c := max(item.X, 0); c < min(item.X+item.Width, GridColumns); c++ {
			skyline[c] = max(skyline[c], item.Y+item.Height)
		}
	}
	bestX, bestY := 0, -1
	for x := 0; x+width <= GridColumns; x++ {
		top := 0
		for c := x; c < x+width; c++ {
			top = max(top, skyline[c])
		}
		if bestY < 0 || top < bestY {
			bestX, bestY = x, top
		}
	}

	item := dashboard.GridItem{X: bestX, Y: bestY, Width: width, Height: height, Content: PanelRef(key)}
	grid.Items = append(grid.Items, item)
	return item, nil
}

// PlacePanelAt adds an item referencing the panel key to the grid layout at an explicit position. A zero width
// or height takes the default.
func PlacePanelAt(layout *dashboard.Layout, key string, x, y, width, height int) (dashboard.GridItem, error) {
	grid, ok := layout.Spec.(*dashboard.GridLayoutSpec)
	if !ok {
		return dashboard.GridItem{}, fmt.Errorf("layout of kind %q is not a grid", layout.Kind)
	}
	if width <= 0 {
		width = DefaultPanelWidth
	}
	if height <= 0 {
		height = DefaultPanelHeight
	}
	item := dashboard.GridItem{X: x, Y: y, Width: width, Height: height, Content: PanelRef(key)}
	grid.Items = append(grid.Items, item)
	return item, nil
}

// RemovePanelItems removes every layout item referencing the panel key and returns the number of items removed.
func RemovePanelItems(layouts []dashboard.Layout, key string) int {
	removed := 0
	filter := func(items []dashboard.GridItem) []dashboard.GridItem {
		kept := items[:0]
		for _, item := range items {
			if panelKeyOf(item) == key {
				removed++
				continue
			}
			kept = append(kept, item)
		}
		return kept
	}
	for _, layout := range layouts {
		switch spec := layout.Spec.(type) {
		case *dashboard.GridLayoutSpec:
			spec.Items = filter(spec.Items)
		case *dashboard.TabLayoutSpec:
			for i := range spec.Tabs {
				spec.Tabs[i].Items = filter(spec.Tabs[i].Items)
			}
		}
	}
	return removed
}
