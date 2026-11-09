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
	"testing"

	dashboardSpec "github.com/perses/spec/go/dashboard"
)

type position struct{ x, y, w, h int }

func positions(items []dashboardSpec.GridItem) []position {
	result := make([]position, 0, len(items))
	for _, item := range items {
		result = append(result, position{item.X, item.Y, item.Width, item.Height})
	}
	return result
}

func TestGridItems(t *testing.T) {
	equal(t, []position{{0, 0, 8, 6}, {8, 0, 8, 6}, {16, 0, 8, 6}, {0, 6, 8, 6}}, positions(gridItems([]string{"a", "b", "c", "d"}, 3, 6)))
	// 5 per line: width 4, never beyond the 24 columns (perses/perses#4716).
	items := gridItems([]string{"a", "b", "c", "d", "e", "f"}, 5, 8)
	equal(t, position{16, 0, 4, 8}, positions(items)[4])
	equal(t, position{0, 8, 4, 8}, positions(items)[5])
	equal(t, "#/spec/panels/f", items[5].Content.Ref)
}

func TestFindGroup(t *testing.T) {
	grid := func(title string) dashboardSpec.Layout {
		return dashboardSpec.Layout{Kind: dashboardSpec.KindGridLayout, Spec: &dashboardSpec.GridLayoutSpec{Display: &dashboardSpec.GridLayoutDisplay{Title: title}}}
	}
	layouts := []dashboardSpec.Layout{grid("R"), grid("r"), grid("Ab"), grid("aB")}
	for title, expected := range map[string]int{"R": 0, "r": 1, "": 3, "Ab": 2, "x": -1} {
		index, err := findGroup(layouts, title)
		if err != nil {
			t.Fatalf("%v: %v", title, err)
		}
		equal(t, expected, index, title)
	}
	_, err := findGroup(layouts, "AB")
	if err == nil || err.Error() != `no Grid group is titled exactly "AB" and several match ignoring case ("Ab", "aB"): give the exact title` {
		t.Errorf("expected error %q, got %v", `no Grid group is titled exactly "AB" and several match ignoring case ("Ab", "aB"): give the exact title`, err)
	}
	index, err := findGroup(layouts[2:3], "AB")
	if err != nil {
		t.Fatal(err)
	}
	equal(t, 0, index, "a single case-insensitive match is accepted")
}

func TestGroupPanels(t *testing.T) {
	keys, perLine, height := groupPanels(&dashboardSpec.GridLayoutSpec{})
	empty(t, keys)
	equal(t, []int{defaultPanelsPerLine, defaultPanelHeight}, []int{perLine, height})

	grid := &dashboardSpec.GridLayoutSpec{Items: gridItems([]string{"a", "b", "c"}, 3, 6)}
	grid.Items[0], grid.Items[2] = grid.Items[2], grid.Items[0]
	keys, perLine, height = groupPanels(grid)
	equal(t, []string{"a", "b", "c"}, keys, "reading order")
	equal(t, []int{3, 6}, []int{perLine, height})

	// A width that does not divide 24 rounds the panels per line down.
	grid = &dashboardSpec.GridLayoutSpec{Items: []dashboardSpec.GridItem{{Width: 7, Height: 5}}}
	_, perLine, _ = groupPanels(grid)
	equal(t, 3, perLine)
}
