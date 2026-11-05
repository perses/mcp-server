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

package validate

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/perses/mcp-server/pkg/tools/internal/proxy"
	datasourceSpec "github.com/perses/spec/go/datasource"
	httpproxy "github.com/perses/spec/go/datasource/proxy/http"
)

const (
	connectivityTimeout = 15 * time.Second
	maxMessageLength    = 300
)

// Connectivity statuses.
const (
	statusOK      = "ok"
	statusTimeout = "timeout"
	statusNoProxy = "no_proxy"
	statusSkipped = "skipped"
	statusError   = "error"
)

// connectivity is the result of querying the datasource through the Perses server.
type connectivity struct {
	Status     string `json:"status"`
	HTTPStatus int    `json:"http_status,omitempty"`
	LatencyMS  int64  `json:"latency_ms,omitempty"`
	Message    string `json:"message,omitempty"`
	Hint       string `json:"hint,omitempty"`
}

// probe is a cheap request telling whether the datasource answers.
type probe struct {
	path   string
	params map[string]string
	// acceptsPost means that the endpoint also accepts its parameters in a POST request, for the datasources whose
	// allowedEndpoints only let POST through.
	acceptsPost bool
	// method is the HTTP method picked to query the datasource.
	method string
}

// probesByKind lists the probes of the known plugins, by preference. vector(1) needs no data, and checks the query
// endpoint that dashboards use, on Prometheus as well as on Thanos, Cortex or Mimir.
var probesByKind = map[string][]probe{
	prometheusKind: {
		{path: "/api/v1/query", params: map[string]string{"query": "vector(1)"}, acceptsPost: true},
		{path: "/api/v1/status/buildinfo"},
		{path: "/api/v1/labels", acceptsPost: true},
	},
	"TempoDatasource": {
		{path: "/api/echo"},
		{path: "/api/search/tags"},
	},
}

var defaultProbes = []probe{{path: "/"}}

// pickProbe returns the first probe that the allowedEndpoints of the datasource let through, with GET when allowed,
// else with POST when the endpoint accepts it. Prometheus reads the parameters of a POST request from the URL too.
func pickProbe(kind string, cfg *httpproxy.Config) (probe, bool) {
	probes, ok := probesByKind[kind]
	if !ok {
		probes = defaultProbes
	}
	for _, candidate := range probes {
		for _, method := range []string{http.MethodGet, http.MethodPost} {
			if method == http.MethodPost && !candidate.acceptsPost {
				continue
			}
			if endpointAllowed(cfg, method, candidate.path) {
				candidate.method = method
				return candidate, true
			}
		}
	}
	return probe{}, false
}

// datasourceRef locates the datasource to query: saved datasources are queried by name, the others through the
// unsaved proxy endpoints of Perses, with their spec in the request body.
type datasourceRef struct {
	scope   string
	project string
	name    string
	saved   bool
	spec    datasourceSpec.Spec
}

func (v *validation) checkConnectivity(ctx context.Context, ref datasourceRef) *connectivity {
	target := httpTargetOf(ref.spec.Plugin)
	if target.proxy == nil {
		if target.directURL != "" {
			return &connectivity{
				Status:  statusNoProxy,
				Message: "the datasource has only a directUrl: the Perses server cannot query it",
				Hint:    "only browsers can query it; add a proxy for server-side queries",
			}
		}
		return &connectivity{Status: statusSkipped, Message: "only HTTP proxy datasources can be checked"}
	}
	selected, ok := pickProbe(ref.spec.Plugin.Kind, target.proxy)
	if !ok {
		return &connectivity{
			Status:  statusSkipped,
			Message: "allowedEndpoints block every endpoint used to check the datasource",
			Hint:    "use perses_query_project_datasource or perses_query_global_datasource with an allowed endpoint",
		}
	}

	if disabled := v.proxyDisabled(ref.scope); disabled != nil {
		return disabled
	}

	query := proxy.ProxyQuery{Method: selected.method, QueryParams: selected.params}
	switch {
	case ref.saved && ref.scope == scopeProject:
		query.Path = proxy.ProjectDatasourceProxyPath(ref.project, ref.name, selected.path)
	case ref.saved:
		query.Path = proxy.GlobalDatasourceProxyPath(ref.name, selected.path)
	default:
		if ref.scope == scopeProject {
			query.Path = proxy.UnsavedProjectDatasourceProxyPath(ref.project, selected.path)
		} else {
			query.Path = proxy.UnsavedGlobalDatasourceProxyPath(selected.path)
		}
		body, err := json.Marshal(proxy.UnsavedBody{Method: selected.method, Spec: ref.spec})
		if err != nil {
			return &connectivity{Status: statusError, Message: fmt.Sprintf("unable to encode the datasource: %s", err)}
		}
		query.Method = http.MethodPost
		query.Body = string(body)
	}

	ctx, cancel := context.WithTimeout(ctx, connectivityTimeout)
	defer cancel()
	start := time.Now()
	resp, err := proxy.New(v.client.V1()).Do(ctx, query)
	latency := time.Since(start).Milliseconds()
	if err != nil {
		if ctx.Err() != nil {
			return &connectivity{Status: statusTimeout, Message: fmt.Sprintf("no answer within %s", connectivityTimeout)}
		}
		return &connectivity{Status: statusError, Message: truncate(fmt.Sprintf("unable to query the Perses server: %s", err))}
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return &connectivity{Status: statusOK, LatencyMS: latency}
	}
	return &connectivity{Status: statusError, HTTPStatus: resp.StatusCode, Message: truncate(errorMessage(string(resp.Body)))}
}

// proxyDisabled reports the datasources whose proxy routes the Perses server does not serve: datasource.global.disable
// and datasource.project.disable remove them, and the datasource would otherwise be reported with the 404 or 405 of
// Perses. When the configuration cannot be read, the proxy is assumed enabled.
func (v *validation) proxyDisabled(scope string) *connectivity {
	cfg, err := v.client.Config()
	if err != nil || cfg == nil {
		return nil
	}
	setting := "datasource.global.disable"
	disabled := cfg.Datasource.Global.Disable
	if scope == scopeProject {
		setting = "datasource.project.disable"
		disabled = cfg.Datasource.Project.Disable
	}
	if !disabled {
		return nil
	}
	return &connectivity{
		Status:  statusSkipped,
		Message: fmt.Sprintf("the Perses server does not serve the %s datasource proxy (%s is set)", scope, setting),
		Hint:    fmt.Sprintf("the Perses server cannot query %s datasources; ask the administrator to unset %s", scope, setting),
	}
}

// errorMessage extracts the message of a Perses error, or returns the body of the datasource answer.
func errorMessage(body string) string {
	var perses struct {
		Message string `json:"message"`
		Error   string `json:"error"`
	}
	if json.Unmarshal([]byte(body), &perses) == nil {
		if perses.Message != "" {
			return perses.Message
		}
		if perses.Error != "" {
			return perses.Error
		}
	}
	return strings.TrimSpace(body)
}

func truncate(message string) string {
	if len(message) <= maxMessageLength {
		return message
	}
	return message[:maxMessageLength] + "…"
}
