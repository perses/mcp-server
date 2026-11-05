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
	"regexp"
	"strings"
	"time"

	"github.com/perses/mcp-server/pkg/tools/internal/proxy"
	datasourcev1 "github.com/perses/perses/pkg/model/api/v1/datasource"
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

// healthCheckPaths are the health endpoints that the Perses UI queries to test the connection of a datasource, by
// plugin kind: the healthCheckPath of the datasource plugins of https://github.com/perses/plugins.
var healthCheckPaths = map[string]string{
	"AlertManagerDatasource": "/api/v2/status",
	"OpenSearchDatasource":   "/_cluster/health",
	"PrometheusDatasource":   "/api/v1/status/buildinfo",
}

// withHealthCheckEndpoint returns a copy of the spec whose HTTP proxy also allows GET on the health endpoint, as the
// Perses UI does before testing the connection. A proxy without allowedEndpoints already allows every endpoint.
func withHealthCheckEndpoint(spec datasourceSpec.Spec, path string) (datasourceSpec.Spec, error) {
	data, err := json.Marshal(spec.Plugin.Spec)
	if err != nil {
		return spec, err
	}
	var pluginSpec map[string]any
	if json.Unmarshal(data, &pluginSpec) != nil {
		return spec, nil
	}
	proxySpec, _ := pluginSpec["proxy"].(map[string]any)
	if kind, _ := proxySpec["kind"].(string); !strings.EqualFold(kind, httpproxy.ProxyKindName) {
		return spec, nil
	}
	httpSpec, _ := proxySpec["spec"].(map[string]any)
	endpoints, _ := httpSpec["allowedEndpoints"].([]any)
	if len(endpoints) == 0 {
		return spec, nil
	}
	pattern := regexp.QuoteMeta(path)
	for _, endpoint := range endpoints {
		if allowed, _ := endpoint.(map[string]any); allowed["endpointPattern"] == pattern && allowed["method"] == http.MethodGet {
			return spec, nil
		}
	}
	httpSpec["allowedEndpoints"] = append(endpoints, map[string]any{"endpointPattern": pattern, "method": http.MethodGet})
	spec.Plugin.Spec = pluginSpec
	return spec, nil
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
	if _, kind, err := datasourcev1.ValidateAndExtract(ref.spec.Plugin.Spec); err != nil || kind != httpproxy.ProxyKindName {
		return &connectivity{Status: statusSkipped, Message: "the datasource has no HTTP proxy: the Perses server cannot query it"}
	}
	path, ok := healthCheckPaths[ref.spec.Plugin.Kind]
	if !ok {
		return &connectivity{Status: statusSkipped, Message: fmt.Sprintf("no health endpoint is known for the %s plugin", ref.spec.Plugin.Kind)}
	}

	if disabled := v.proxyDisabled(ref.scope); disabled != nil {
		return disabled
	}

	query := proxy.ProxyQuery{Method: http.MethodGet}
	switch {
	case ref.saved && ref.scope == scopeProject:
		query.Path = proxy.ProjectDatasourceProxyPath(ref.project, ref.name, path)
	case ref.saved:
		query.Path = proxy.GlobalDatasourceProxyPath(ref.name, path)
	default:
		if ref.scope == scopeProject {
			query.Path = proxy.UnsavedProjectDatasourceProxyPath(ref.project, path)
		} else {
			query.Path = proxy.UnsavedGlobalDatasourceProxyPath(path)
		}
		spec, err := withHealthCheckEndpoint(ref.spec, path)
		if err != nil {
			return &connectivity{Status: statusError, Message: fmt.Sprintf("unable to encode the datasource: %s", err)}
		}
		body, err := json.Marshal(proxy.UnsavedBody{Method: http.MethodGet, Spec: spec})
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
