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
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	apiClient "github.com/perses/perses/pkg/client/api/v1"
	"github.com/perses/spec/go/datasource"
)

// prometheusDatasourceKind is the plugin kind of Prometheus datasources.
const prometheusDatasourceKind = "PrometheusDatasource"

// cacheTTL is how long a resolved datasource is reused.
const cacheTTL = 30 * time.Second

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

type cacheEntry struct {
	ref     DatasourceRef
	expires time.Time
}

// resolver resolves a (possibly partial) datasource reference to an existing Prometheus datasource.
//
// The resolved references are cached for cacheTTL and shared by all the tool calls. This is right as long as the
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
// When no Prometheus datasource is marked as default but there is exactly one (project and global), it is used.
func (r *resolver) resolve(input *DatasourceRef) (DatasourceRef, error) {
	var key DatasourceRef
	if input != nil {
		key = *input
	}
	r.mutex.Lock()
	entry, ok := r.cache[key]
	r.mutex.Unlock()
	if ok && r.now().Before(entry.expires) {
		return entry.ref, nil
	}
	ref, err := r.resolveUncached(key)
	if err != nil {
		return DatasourceRef{}, err
	}
	r.mutex.Lock()
	r.cache[key] = cacheEntry{ref: ref, expires: r.now().Add(cacheTTL)}
	r.mutex.Unlock()
	return ref, nil
}

func (r *resolver) resolveUncached(key DatasourceRef) (DatasourceRef, error) {
	if key.Name != "" {
		return r.resolveNamed(key)
	}
	var available []DatasourceRef
	if key.Project != "" {
		datasources, err := r.client.Datasource(key.Project).List("")
		if err != nil {
			return DatasourceRef{}, fmt.Errorf("error listing the datasources of project '%s': %w", key.Project, err)
		}
		candidates := map[string]datasource.Spec{}
		for _, ds := range datasources {
			candidates[ds.Metadata.Name] = ds.Spec
		}
		if name, ok := pickDefault(candidates); ok {
			return DatasourceRef{Name: name, Project: key.Project}, nil
		}
		for _, name := range prometheusNames(candidates) {
			available = append(available, DatasourceRef{Name: name, Project: key.Project})
		}
	}
	globals, err := r.client.GlobalDatasource().List("")
	if err != nil {
		return DatasourceRef{}, fmt.Errorf("error listing the global datasources: %w", err)
	}
	candidates := map[string]datasource.Spec{}
	for _, ds := range globals {
		candidates[ds.Metadata.Name] = ds.Spec
	}
	if name, ok := pickDefault(candidates); ok {
		return DatasourceRef{Name: name}, nil
	}
	for _, name := range prometheusNames(candidates) {
		available = append(available, DatasourceRef{Name: name})
	}
	switch len(available) {
	case 0:
		return DatasourceRef{}, errors.New("no Prometheus datasource found: create one in Perses or pass datasource.name")
	case 1:
		// No default but a single Prometheus datasource: no ambiguity.
		return available[0], nil
	}
	names := make([]string, 0, len(available))
	for _, ref := range available {
		if ref.Project == "" {
			names = append(names, ref.Name+" (global)")
		} else {
			names = append(names, fmt.Sprintf("%s (project %s)", ref.Name, ref.Project))
		}
	}
	return DatasourceRef{}, fmt.Errorf("no default Prometheus datasource found: pass datasource.name (and datasource.project for a project datasource), available Prometheus datasources: %s",
		strings.Join(names, ", "))
}

func (r *resolver) resolveNamed(key DatasourceRef) (DatasourceRef, error) {
	var spec datasource.Spec
	if key.Project != "" {
		ds, err := r.client.Datasource(key.Project).Get(key.Name)
		if err != nil {
			return DatasourceRef{}, fmt.Errorf("error retrieving %s: %w", key, err)
		}
		spec = ds.Spec
	} else {
		ds, err := r.client.GlobalDatasource().Get(key.Name)
		if err != nil {
			return DatasourceRef{}, fmt.Errorf("error retrieving %s (set datasource.project for a project datasource): %w", key, err)
		}
		spec = ds.Spec
	}
	if spec.Plugin.Kind != prometheusDatasourceKind {
		return DatasourceRef{}, fmt.Errorf("%s is a %s, not a %s", key, spec.Plugin.Kind, prometheusDatasourceKind)
	}
	return key, nil
}

// pickDefault returns the Prometheus datasource marked as default.
func pickDefault(candidates map[string]datasource.Spec) (string, bool) {
	for _, name := range prometheusNames(candidates) {
		if candidates[name].Default {
			return name, true
		}
	}
	return "", false
}

// prometheusNames returns the sorted names of the Prometheus datasources.
func prometheusNames(candidates map[string]datasource.Spec) []string {
	var names []string
	for name, spec := range candidates {
		if spec.Plugin.Kind == prometheusDatasourceKind {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}
