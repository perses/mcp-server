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

package prometheus

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	apiClient "github.com/perses/perses/pkg/client/api/v1"
	promDatasource "github.com/perses/plugins/prometheus/sdk/go/datasource"
	"github.com/perses/spec/go/datasource"
)

// prometheusDatasourceKind is the plugin kind of Prometheus datasources.
const prometheusDatasourceKind = promDatasource.PluginKind

// defaultScrapeInterval is the scrape interval of a datasource that doesn't set one, as in the Perses UI.
const defaultScrapeInterval = time.Minute

// cacheTTL is how long a resolved datasource is reused.
const cacheTTL = 30 * time.Second

// errNoProxy tells that a datasource can't be queried through the Perses proxy.
var errNoProxy = errors.New("the MCP server queries datasources through the Perses proxy: configure a proxy on it")

// DatasourceRef references a Prometheus datasource. Project is empty for a global datasource. In the tool inputs,
// Name is optional: the default Prometheus datasource is used when it is empty.
type DatasourceRef struct {
	Name    string `json:"name"`
	Project string `json:"project,omitempty"`
}

func (d DatasourceRef) String() string {
	if d.Project == "" {
		return fmt.Sprintf("global datasource '%s'", d.Name)
	}
	return fmt.Sprintf("datasource '%s' of project '%s'", d.Name, d.Project)
}

// label returns the reference as listed in errors, e.g. "prom (global)".
func (d DatasourceRef) label() string {
	if d.Project == "" {
		return d.Name + " (global)"
	}
	return fmt.Sprintf("%s (project %s)", d.Name, d.Project)
}

// resolvedDatasource is a Prometheus datasource to query, with the settings of its plugin spec that the Perses UI
// applies to its requests (the Perses proxy doesn't).
type resolvedDatasource struct {
	ref DatasourceRef
	// queryParams are added to every request, unless the request sets the same parameter.
	queryParams map[string]string
	// scrapeInterval is the minimum step of range queries.
	scrapeInterval time.Duration
}

// warnings returns the warnings about the settings of the datasource that the MCP server can't apply as the UI does.
func (d resolvedDatasource) warnings() []string {
	var warnings []string
	for _, key := range slices.Sorted(maps.Keys(d.queryParams)) {
		if strings.Contains(d.queryParams[key], "$") {
			warnings = append(warnings, fmt.Sprintf("the query parameter '%s' of %s may contain a dashboard variable, "+
				"which is sent unresolved: %s", key, d.ref, d.queryParams[key]))
		}
	}
	return warnings
}

// newResolvedDatasource reads the plugin spec of a Prometheus datasource. It fails with errNoProxy when the
// datasource is only reachable by the browser (directUrl).
func newResolvedDatasource(ref DatasourceRef, spec datasource.Spec) (resolvedDatasource, error) {
	pluginSpec := spec.Plugin.Spec
	if fields, isMap := pluginSpec.(map[string]any); isMap && fields["scrapeInterval"] == "" {
		// Perses accepts an empty scrape interval (the UI saves it when the field is cleared), which means the
		// default one, but the SDK rejects it.
		withoutInterval := maps.Clone(fields)
		delete(withoutInterval, "scrapeInterval")
		pluginSpec = withoutInterval
	}
	data, err := json.Marshal(pluginSpec)
	if err != nil {
		return resolvedDatasource{}, fmt.Errorf("error reading the plugin spec of %s: %w", ref, err)
	}
	var plugin promDatasource.PluginSpec
	if err = json.Unmarshal(data, &plugin); err != nil {
		return resolvedDatasource{}, fmt.Errorf("invalid plugin spec of %s: %w", ref, err)
	}
	if plugin.Proxy == nil {
		return resolvedDatasource{}, fmt.Errorf("%s uses a directUrl, reachable by the browser only: %w", ref, errNoProxy)
	}
	scrapeInterval := time.Duration(plugin.ScrapeInterval)
	if scrapeInterval <= 0 {
		scrapeInterval = defaultScrapeInterval
	}
	return resolvedDatasource{ref: ref, queryParams: plugin.QueryParams, scrapeInterval: scrapeInterval}, nil
}

type cacheEntry struct {
	datasource resolvedDatasource
	expires    time.Time
}

// resolver resolves a (possibly partial) datasource reference to an existing Prometheus datasource.
//
// The resolved datasources are cached for cacheTTL and shared by all the tool calls. This is right as long as the
// MCP server queries Perses with a single identity (the credentials of its configuration): with per-user
// credentials, the cache must be keyed by user, as users may see different datasources.
type resolver struct {
	client apiClient.ClientInterface
	now    func() time.Time
	mutex  sync.Mutex
	cache  map[DatasourceRef]cacheEntry
}

func newResolver(client apiClient.ClientInterface) *resolver {
	return &resolver{client: client, now: time.Now, cache: map[DatasourceRef]cacheEntry{}}
}

// resolve returns the datasource to query:
//   - name given: that datasource (project datasource when project is set, global otherwise), which must be a
//     Prometheus datasource;
//   - name omitted, project given: the default Prometheus datasource of the project, else the global one;
//   - name and project omitted: the default global Prometheus datasource.
//
// When no Prometheus datasource is marked as default but there is exactly one that the Perses proxy can query
// (project and global), it is used. The datasource must have a proxy: a directUrl is only reachable by the browser.
func (r *resolver) resolve(input *DatasourceRef) (resolvedDatasource, error) {
	var key DatasourceRef
	if input != nil {
		key = *input
	}
	r.mutex.Lock()
	entry, ok := r.cache[key]
	r.mutex.Unlock()
	if ok && r.now().Before(entry.expires) {
		return entry.datasource, nil
	}
	resolved, err := r.resolveUncached(key)
	if err != nil {
		return resolvedDatasource{}, err
	}
	r.mutex.Lock()
	r.cache[key] = cacheEntry{datasource: resolved, expires: r.now().Add(cacheTTL)}
	r.mutex.Unlock()
	return resolved, nil
}

// candidate is a Prometheus datasource that may be the default one.
type candidate struct {
	ref  DatasourceRef
	spec datasource.Spec
}

func (r *resolver) resolveUncached(key DatasourceRef) (resolvedDatasource, error) {
	if key.Name != "" {
		return r.resolveNamed(key)
	}
	// The default of the project wins over the global one: the global datasources are only listed without it.
	var candidates []candidate
	if key.Project != "" {
		datasources, err := r.client.Datasource(key.Project).List("")
		if err != nil {
			return resolvedDatasource{}, fmt.Errorf("error listing the datasources of project '%s': %w", key.Project, err)
		}
		specs := map[string]datasource.Spec{}
		for _, ds := range datasources {
			specs[ds.Metadata.Name] = ds.Spec
		}
		projectCandidates := prometheusCandidates(specs, key.Project)
		if resolved, found, err := resolveDefault(projectCandidates); found {
			return resolved, err
		}
		candidates = projectCandidates
	}
	globals, err := r.client.GlobalDatasource().List("")
	if err != nil {
		return resolvedDatasource{}, fmt.Errorf("error listing the global datasources: %w", err)
	}
	specs := map[string]datasource.Spec{}
	for _, ds := range globals {
		specs[ds.Metadata.Name] = ds.Spec
	}
	globalCandidates := prometheusCandidates(specs, "")
	if resolved, found, err := resolveDefault(globalCandidates); found {
		return resolved, err
	}
	candidates = append(candidates, globalCandidates...)

	// No default: the only datasource that can be queried, if any.
	var usable []resolvedDatasource
	var available, unusable []string
	for _, c := range candidates {
		resolved, resolveErr := newResolvedDatasource(c.ref, c.spec)
		if resolveErr != nil {
			unusable = append(unusable, resolveErr.Error())
			continue
		}
		usable = append(usable, resolved)
		available = append(available, c.ref.label())
	}
	switch len(usable) {
	case 0:
		if len(unusable) > 0 {
			return resolvedDatasource{}, fmt.Errorf("no Prometheus datasource can be used: %s", strings.Join(unusable, "; "))
		}
		return resolvedDatasource{}, errors.New("no Prometheus datasource found: create one in Perses or pass datasource.name")
	case 1:
		// No default but a single Prometheus datasource that can be queried: no ambiguity.
		return usable[0], nil
	}
	return resolvedDatasource{}, fmt.Errorf("no default Prometheus datasource found: pass datasource.name (and datasource.project for a project datasource), available Prometheus datasources: %s",
		strings.Join(available, ", "))
}

// resolveDefault resolves the datasource marked as default among candidates, if any (found). The default is the one
// users expect: when it can't be used, the error is returned rather than another datasource picked.
func resolveDefault(candidates []candidate) (resolved resolvedDatasource, found bool, err error) {
	for _, c := range candidates {
		if c.spec.Default {
			resolved, err = newResolvedDatasource(c.ref, c.spec)
			if err != nil {
				return resolvedDatasource{}, true, fmt.Errorf("the default Prometheus datasource can't be used: %w", err)
			}
			return resolved, true, nil
		}
	}
	return resolvedDatasource{}, false, nil
}

func (r *resolver) resolveNamed(key DatasourceRef) (resolvedDatasource, error) {
	var spec datasource.Spec
	if key.Project != "" {
		ds, err := r.client.Datasource(key.Project).Get(key.Name)
		if err != nil {
			return resolvedDatasource{}, fmt.Errorf("error retrieving %s: %w", key, err)
		}
		spec = ds.Spec
	} else {
		ds, err := r.client.GlobalDatasource().Get(key.Name)
		if err != nil {
			return resolvedDatasource{}, fmt.Errorf("error retrieving %s (set datasource.project for a project datasource): %w", key, err)
		}
		spec = ds.Spec
	}
	if spec.Plugin.Kind != prometheusDatasourceKind {
		return resolvedDatasource{}, fmt.Errorf("%s is a %s, not a %s", key, spec.Plugin.Kind, prometheusDatasourceKind)
	}
	return newResolvedDatasource(key, spec)
}

// prometheusCandidates returns the Prometheus datasources of a scope (project, or global when empty), sorted by name.
func prometheusCandidates(specs map[string]datasource.Spec, project string) []candidate {
	var candidates []candidate
	for name, spec := range specs {
		if spec.Plugin.Kind == prometheusDatasourceKind {
			candidates = append(candidates, candidate{ref: DatasourceRef{Name: name, Project: project}, spec: spec})
		}
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].ref.Name < candidates[j].ref.Name })
	return candidates
}
