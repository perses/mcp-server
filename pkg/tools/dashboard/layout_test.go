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
		index, spec, err := findGroup(layouts, title)
		if err != nil {
			t.Fatalf("%v: %v", title, err)
		}
		equal(t, expected, index, title)
		if index >= 0 && spec != layouts[index].Spec {
			t.Errorf("%v: findGroup must return the spec of layout %d", title, index)
		}
		if index < 0 && spec != nil {
			t.Errorf("%v: expected no spec, got %#v", title, spec)
		}
	}
	_, _, err := findGroup(layouts, "AB")
	if err == nil || err.Error() != `no Grid group is titled exactly "AB" and several match ignoring case ("Ab", "aB"): give the exact title` {
		t.Errorf("expected error %q, got %v", `no Grid group is titled exactly "AB" and several match ignoring case ("Ab", "aB"): give the exact title`, err)
	}
	index, spec, err := findGroup(layouts[2:3], "AB")
	if err != nil {
		t.Fatal(err)
	}
	equal(t, 0, index, "a single case-insensitive match is accepted")
	if spec != layouts[2].Spec {
		t.Error("findGroup must return the spec of the case-insensitive match")
	}
}

func TestFollowsSDK(t *testing.T) {
	sdk := &dashboardSpec.GridLayoutSpec{Items: gridItems([]string{"a", "b", "c"}, 3, 6)}
	sdk.Items[0], sdk.Items[2] = sdk.Items[2], sdk.Items[0]
	equal(t, true, followsSDK(sdk), "the item order does not matter")
	equal(t, true, followsSDK(&dashboardSpec.GridLayoutSpec{}), "an empty group")

	moved := &dashboardSpec.GridLayoutSpec{Items: gridItems([]string{"a", "b"}, 2, 8)}
	moved.Items[1].Y = 3
	equal(t, false, followsSDK(moved), "a moved panel")

	resized := &dashboardSpec.GridLayoutSpec{Items: gridItems([]string{"a", "b"}, 2, 8)}
	resized.Items[1].Height = 4
	equal(t, false, followsSDK(resized), "a resized panel")

	// A width that does not divide 24 is not what gridItems produces.
	equal(t, false, followsSDK(&dashboardSpec.GridLayoutSpec{Items: []dashboardSpec.GridItem{
		{Width: 10, Height: 6, Content: gridItems([]string{"a"}, 1, 1)[0].Content},
	}}))
}

func TestAppendBelow(t *testing.T) {
	grid := &dashboardSpec.GridLayoutSpec{Items: []dashboardSpec.GridItem{
		{X: 4, Y: 0, Width: 10, Height: 5, Content: gridItems([]string{"a"}, 1, 1)[0].Content},
		{X: 0, Y: 3, Width: 6, Height: 9, Content: gridItems([]string{"b"}, 1, 1)[0].Content},
	}}
	appendBelow(grid, []string{"c", "d", "e"})
	equal(t, []position{{4, 0, 10, 5}, {0, 3, 6, 9}, {0, 12, 10, 5}, {10, 12, 10, 5}, {0, 17, 10, 5}}, positions(grid.Items),
		"existing items stay, new ones take the size of the first item below the lowest one")
	equal(t, "#/spec/panels/e", grid.Items[4].Content.Ref)
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
