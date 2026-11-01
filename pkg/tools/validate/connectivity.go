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
	statusOK          = "ok"
	statusDNS         = "dns"
	statusUnreachable = "unreachable"
	statusTimeout     = "timeout"
	statusTLS         = "tls"
	statusAuth        = "auth"
	statusBlocked     = "blocked"
	statusForbidden   = "forbidden"
	statusNoProxy     = "no_proxy"
	statusSkipped     = "skipped"
	statusError       = "error"
)

// connectivity is the result of querying the datasource through the Perses server.
type connectivity struct {
	Status    string `json:"status"`
	LatencyMS int64  `json:"latency_ms,omitempty"`
	Message   string `json:"message,omitempty"`
	Hint      string `json:"hint,omitempty"`
}

// probe is a cheap request telling whether the datasource answers.
type probe struct {
	path   string
	params map[string]string
	// anyResponse means that any response of the datasource, even an error, proves that it is reachable.
	anyResponse bool
}

// probesByKind lists the probes of the known plugins, by preference. vector(1) needs no data, and checks the query
// endpoint that dashboards use, on Prometheus as well as on Thanos, Cortex or Mimir.
var probesByKind = map[string][]probe{
	prometheusKind: {
		{path: "/api/v1/query", params: map[string]string{"query": "vector(1)"}},
		{path: "/api/v1/status/buildinfo"},
		{path: "/api/v1/labels"},
	},
	"TempoDatasource": {
		{path: "/api/echo"},
		{path: "/api/search/tags"},
	},
}

var defaultProbes = []probe{{path: "/", anyResponse: true}}

// pickProbe returns the first probe that the allowedEndpoints of the datasource let through.
func pickProbe(kind string, cfg *httpproxy.Config) (probe, bool) {
	probes, ok := probesByKind[kind]
	if !ok {
		probes = defaultProbes
	}
	for _, candidate := range probes {
		if endpointAllowed(cfg, http.MethodGet, candidate.path) {
			return candidate, true
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

	query := proxy.ProxyQuery{Method: http.MethodGet, QueryParams: selected.params}
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
		body, err := json.Marshal(proxy.UnsavedBody{Method: http.MethodGet, Spec: ref.spec})
		if err != nil {
			return &connectivity{Status: statusError, Message: fmt.Sprintf("unable to encode the datasource: %s", err)}
		}
		query.Method = http.MethodPost
		query.Body = string(body)
	}

	ctx, cancel := context.WithTimeout(ctx, connectivityTimeout)
	defer cancel()
	start := time.Now()
	result, err := proxy.New(v.client.V1()).Do(ctx, query)
	latency := time.Since(start).Milliseconds()
	if err != nil {
		if ctx.Err() != nil {
			return &connectivity{Status: statusTimeout, Message: fmt.Sprintf("no answer within %s", connectivityTimeout)}
		}
		return &connectivity{Status: statusError, Message: truncate(fmt.Sprintf("unable to query the Perses server: %s", err))}
	}
	check := classify(result.StatusCode, result.Body, selected.anyResponse, ref.saved)
	if check.Status == statusOK {
		check.LatencyMS = latency
	}
	return check
}

// classify interprets the response of the Perses datasource proxy. The errors of the Perses server itself (proxy
// configuration, permissions, network errors) are distinguished by their message from the answers of the datasource.
func classify(status int, body string, anyResponse, saved bool) *connectivity {
	if status >= 200 && status < 300 {
		return &connectivity{Status: statusOK}
	}
	message := errorMessage(body)
	lower := strings.ToLower(message)
	result := func(s, hint string) *connectivity {
		return &connectivity{Status: s, Message: truncate(fmt.Sprintf("HTTP %d: %s", status, message)), Hint: hint}
	}
	switch {
	case strings.Contains(lower, "not allowed to use this endpoint"):
		return result(statusBlocked, "allowedEndpoints of the datasource proxy block this endpoint")
	case strings.Contains(lower, "destination is not allowed"):
		return result(statusBlocked, "the Perses server configuration (datasource.proxy) does not allow this destination")
	case status == http.StatusForbidden && strings.Contains(lower, "permission"):
		if saved {
			return result(statusForbidden, "the Perses user of the MCP server cannot read this datasource")
		}
		return result(statusForbidden, "checking an unsaved datasource requires the permission to create datasources; "+
			"save it, then check it by name")
	case strings.Contains(lower, "no proxy kind found"):
		return result(statusNoProxy, "the Perses server cannot query a datasource without proxy")
	case strings.Contains(lower, "no such host"):
		return result(statusDNS, "the Perses server cannot resolve this hostname; use one that it can resolve")
	case strings.Contains(lower, "connection refused"), strings.Contains(lower, "no route to host"),
		strings.Contains(lower, "network is unreachable"):
		return result(statusUnreachable, "nothing listens at this address from the Perses server; check the host and port")
	case strings.Contains(lower, "timeout"), strings.Contains(lower, "deadline exceeded"), status == http.StatusGatewayTimeout:
		return result(statusTimeout, "the datasource did not answer in time from the Perses server")
	case strings.Contains(lower, "x509"), strings.Contains(lower, "tls:"), strings.Contains(lower, "certificate"):
		return result(statusTLS, "check the scheme (http/https) and the TLS configuration of the proxy")
	case strings.Contains(lower, "doesn't exist"):
		return result(statusError, "")
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		return result(statusAuth, "the datasource rejected the credentials; check the secret of the proxy")
	case anyResponse && status < 500:
		return &connectivity{Status: statusOK}
	case status == http.StatusNotFound:
		return result(statusError, "the endpoint was not found; check the path of the URL (e.g. a missing or extra prefix)")
	}
	return result(statusError, "")
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
