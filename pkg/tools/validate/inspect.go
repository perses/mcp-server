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
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/perses/mcp-server/pkg/tools/internal/dashboardutil"
	datasourcev1 "github.com/perses/perses/pkg/model/api/v1/datasource"
	datasourceSpec "github.com/perses/spec/go/datasource"
	httpproxy "github.com/perses/spec/go/datasource/proxy/http"
	"github.com/perses/spec/go/plugin"
)

const prometheusKind = "PrometheusDatasource"

// prometheusDashboardEndpoints are the Prometheus API endpoints that the Perses UI queries for dashboards.
var prometheusDashboardEndpoints = []string{
	"/api/v1/query",
	"/api/v1/query_range",
	"/api/v1/labels",
	"/api/v1/series",
	"/api/v1/label/job/values",
}

// httpTarget is how Perses reaches an HTTP datasource.
type httpTarget struct {
	// directURL is set when only the browser queries the datasource.
	directURL string
	// proxy is set when the Perses server queries the datasource.
	proxy *httpproxy.Config
	// proxyPath is the JSON path of the proxy configuration.
	proxyPath string
}

// httpTargetOf finds how the datasource is reached. Most HTTP datasource plugins (Prometheus, Tempo, Loki, ...)
// follow datasource.HTTPDatasourceSpec; the others are searched for an HTTP proxy the way the Perses server does.
func httpTargetOf(p plugin.Plugin) httpTarget {
	if data, err := json.Marshal(p.Spec); err == nil {
		var spec datasourceSpec.HTTPDatasourceSpec
		if json.Unmarshal(data, &spec) == nil {
			target := httpTarget{directURL: spec.DirectURL, proxyPath: "spec.plugin.spec.proxy.spec"}
			if spec.Proxy != nil && strings.EqualFold(spec.Proxy.Kind, httpproxy.ProxyKindName) {
				target.proxy = &spec.Proxy.Spec
			}
			return target
		}
	}
	if cfg, kind, err := datasourcev1.ValidateAndExtract(p.Spec); err == nil && kind == httpproxy.ProxyKindName {
		if httpConfig, ok := cfg.(*httpproxy.Config); ok {
			return httpTarget{proxy: httpConfig, proxyPath: "spec.plugin.spec"}
		}
	}
	return httpTarget{}
}

// lintDatasource reports the datasource configurations that Perses accepts but that likely do not work.
func lintDatasource(spec datasourceSpec.Spec) []dashboardutil.Issue {
	target := httpTargetOf(spec.Plugin)
	var warnings []dashboardutil.Issue
	if target.directURL != "" && target.proxy == nil {
		warnings = append(warnings, dashboardutil.Issue{
			Path: "spec.plugin.spec.directUrl",
			Message: "only browsers query this datasource: without a proxy, server-side queries (Perses datasource proxy, " +
				"perses_query_*_datasource tools, connectivity check) fail",
			Hint: `use "proxy": {"kind": "HTTPProxy", "spec": {"url": "..."}} instead, unless browsers must query it directly`,
		})
	}
	if target.proxy == nil {
		return warnings
	}
	if u := target.proxy.URL; u != nil && u.URL != nil {
		if isLoopback(u.Hostname()) {
			warnings = append(warnings, dashboardutil.Issue{
				Path:    target.proxyPath + ".url",
				Message: fmt.Sprintf("%q is resolved by the Perses server, not by the machine of the user", u.Hostname()),
				Hint:    "use a hostname that the Perses server can reach",
			})
		}
		if spec.Plugin.Kind == prometheusKind && strings.HasSuffix(strings.TrimRight(u.Path, "/"), "/api/v1") {
			warnings = append(warnings, dashboardutil.Issue{
				Path:    target.proxyPath + ".url",
				Message: "the URL ends with /api/v1, which the queries add again",
				Hint:    "remove /api/v1 from the URL",
			})
		}
	}
	if spec.Plugin.Kind == prometheusKind {
		var blocked []string
		for _, endpoint := range prometheusDashboardEndpoints {
			if !endpointAllowed(target.proxy, http.MethodGet, endpoint) && !endpointAllowed(target.proxy, http.MethodPost, endpoint) {
				blocked = append(blocked, endpoint)
			}
		}
		if len(blocked) > 0 {
			warnings = append(warnings, dashboardutil.Issue{
				Path:    target.proxyPath + ".allowedEndpoints",
				Message: "dashboards cannot use " + strings.Join(blocked, ", "),
				Hint:    "allow these endpoints (GET or POST), or remove allowedEndpoints to allow every endpoint",
			})
		}
	}
	return warnings
}

// endpointAllowed mirrors the check of the Perses HTTP proxy: without allowedEndpoints, every endpoint is allowed.
func endpointAllowed(cfg *httpproxy.Config, method, path string) bool {
	if len(cfg.AllowedEndpoints) == 0 {
		return true
	}
	for _, endpoint := range cfg.AllowedEndpoints {
		if endpoint.Method == method && endpoint.EndpointPattern.Regexp != nil && endpoint.EndpointPattern.MatchString(path) {
			return true
		}
	}
	return false
}

func isLoopback(host string) bool {
	if strings.EqualFold(host, "localhost") || strings.HasSuffix(strings.ToLower(host), ".localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
