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
	"net/url"
	"sort"
	"strings"

	v1 "github.com/perses/perses/pkg/model/api/v1"
	dashboardSpec "github.com/perses/spec/go/dashboard"
)

// Summary is the compact description of a dashboard returned by the write tools instead of the whole dashboard.
type Summary struct {
	Project string `json:"project"`
	Name    string `json:"name"`
	Display string `json:"display,omitempty"`
	Version uint64 `json:"version"`
	// URL is the dashboard page in the Perses UI.
	URL       string `json:"url,omitempty"`
	Panels    int    `json:"panels"`
	Variables int    `json:"variables"`
	Layouts   int    `json:"layouts"`
}

// Outline describes the structure of a dashboard without its plugin specs: enough to decide which panel, variable
// or layout to change next.
type Outline struct {
	Summary
	Duration        string            `json:"duration,omitempty"`
	RefreshInterval string            `json:"refreshInterval,omitempty"`
	Datasources     []string          `json:"datasources,omitempty"`
	VariableList    []VariableOutline `json:"variableList,omitempty"`
	PanelList       []PanelOutline    `json:"panelList,omitempty"`
	LayoutList      []LayoutOutline   `json:"layoutList,omitempty"`
}

// PanelOutline describes a panel and where it is displayed.
type PanelOutline struct {
	Key     string `json:"key"`
	Name    string `json:"name,omitempty"`
	Plugin  string `json:"plugin"`
	Queries int    `json:"queries"`
	// Placement lists the layout items displaying the panel, as "layout <index>: x=.. y=.. w=.. h=..".
	Placement []string `json:"placement,omitempty"`
}

// VariableOutline describes a variable.
type VariableOutline struct {
	Name   string `json:"name"`
	Kind   string `json:"kind"`
	Plugin string `json:"plugin,omitempty"`
}

// LayoutOutline describes a layout group.
type LayoutOutline struct {
	Index  int      `json:"index"`
	Kind   string   `json:"kind"`
	Title  string   `json:"title,omitempty"`
	Items  int      `json:"items"`
	Panels []string `json:"panels,omitempty"`
}

// DashboardURL returns the page of the dashboard in the Perses UI served at the base URL. Credentials embedded in
// the base URL are dropped so links shown to clients never expose them; base itself is left unchanged.
func DashboardURL(base *url.URL, project, name string) string {
	if base == nil {
		return ""
	}
	page := *base
	page.User = nil
	page.Path = strings.TrimRight(page.Path, "/") + "/projects/" + url.PathEscape(project) + "/dashboards/" + url.PathEscape(name)
	page.RawQuery = ""
	page.Fragment = ""
	return page.String()
}

// Summarize builds the summary of a dashboard of the project. The project is passed explicitly because the
// metadata.project of a dashboard not saved yet can be empty.
func Summarize(d *v1.Dashboard, project string, base *url.URL) Summary {
	summary := Summary{
		Project:   project,
		Name:      d.Metadata.Name,
		Version:   d.Metadata.Version,
		URL:       DashboardURL(base, project, d.Metadata.Name),
		Panels:    len(d.Spec.Panels),
		Variables: len(d.Spec.Variables),
		Layouts:   len(d.Spec.Layouts),
	}
	if d.Spec.Display != nil {
		summary.Display = d.Spec.Display.Name
	}
	return summary
}

// OutlineOf builds the outline of a saved dashboard.
func OutlineOf(d *v1.Dashboard, base *url.URL) Outline {
	outline := Outline{
		Summary:         Summarize(d, d.Metadata.Project, base),
		Duration:        string(d.Spec.Duration),
		RefreshInterval: string(d.Spec.RefreshInterval),
	}
	for name := range d.Spec.Datasources {
		outline.Datasources = append(outline.Datasources, name)
	}
	sort.Strings(outline.Datasources)

	for _, variable := range d.Spec.Variables {
		item := VariableOutline{Kind: string(variable.Kind)}
		if variable.Spec != nil {
			item.Name = variable.Spec.GetName()
		}
		if spec, ok := variable.Spec.(*dashboardSpec.ListVariableSpec); ok {
			item.Plugin = spec.Plugin.Kind
		}
		outline.VariableList = append(outline.VariableList, item)
	}

	placements := map[string][]string{}
	for i, layout := range d.Spec.Layouts {
		group := LayoutOutline{Index: i, Kind: string(layout.Kind), Title: layoutTitle(layout)}
		for _, items := range itemGroups(layout) {
			for _, item := range items {
				group.Items++
				key := panelKeyOf(item)
				if key == "" {
					continue
				}
				group.Panels = append(group.Panels, key)
				placements[key] = append(placements[key], fmt.Sprintf("layout %d: x=%d y=%d w=%d h=%d", i, item.X, item.Y, item.Width, item.Height))
			}
		}
		outline.LayoutList = append(outline.LayoutList, group)
	}

	for _, key := range sortedPanelKeys(d) {
		panel := d.Spec.Panels[key]
		item := PanelOutline{Key: key, Plugin: panel.Spec.Plugin.Kind, Queries: len(panel.Spec.Queries), Placement: placements[key]}
		if panel.Spec.Display != nil {
			item.Name = panel.Spec.Display.Name
		}
		outline.PanelList = append(outline.PanelList, item)
	}
	return outline
}

func sortedPanelKeys(d *v1.Dashboard) []string {
	keys := make([]string, 0, len(d.Spec.Panels))
	for key, panel := range d.Spec.Panels {
		if panel != nil {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	return keys
}
