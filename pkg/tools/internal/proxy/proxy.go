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

package proxy

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	apiClient "github.com/perses/perses/pkg/client/api/v1"
)

// maxBodyBytes caps response body size to prevent overwhelming LLM context windows.
const maxBodyBytes = 1 << 20 // 1 MiB

// blockedInputHeaders lists headers callers must not override; the Perses proxy
// manages auth via the configured Secret, not via tool input.
var blockedInputHeaders = map[string]struct{}{
	"authorization": {},
}

// These mirror perses/internal/api/utils constants which are not importable.
const (
	pathProxy             = "proxy"
	pathProjects          = "projects"
	pathDatasources       = "datasources"
	pathGlobalDatasources = "globaldatasources"
)

type ProxyQuery struct {
	Method      string
	Path        string
	QueryParams map[string]string
	Body        string
	Headers     map[string]string
}

// ProjectDatasourceProxyPath builds the Perses proxy sub-path for a project-scoped datasource.
// subPath is the datasource-relative endpoint path (e.g. /api/v1/query_range).
func ProjectDatasourceProxyPath(project, datasource, subPath string) string {
	return fmt.Sprintf("/%s/%s/%s/%s/%s%s",
		pathProxy,
		pathProjects, url.PathEscape(project),
		pathDatasources, url.PathEscape(datasource),
		NormalizePath(subPath))
}

// GlobalDatasourceProxyPath builds the Perses proxy sub-path for a global datasource.
// subPath is the datasource-relative endpoint path (e.g. /api/v1/query_range).
func GlobalDatasourceProxyPath(datasource, subPath string) string {
	return fmt.Sprintf("/%s/%s/%s%s",
		pathProxy,
		pathGlobalDatasources, url.PathEscape(datasource),
		NormalizePath(subPath))
}

type QueryResult struct {
	StatusCode int               `json:"status_code"`
	Headers    map[string]string `json:"headers,omitempty"`
	Body       string            `json:"body,omitempty"`
}

type Helper struct {
	client apiClient.ClientInterface
}

func New(client apiClient.ClientInterface) *Helper {
	return &Helper{client: client}
}

func NormalizeMethod(method string) string {
	method = strings.ToUpper(strings.TrimSpace(method))
	if method == "" {
		return http.MethodGet
	}
	return method
}

func NormalizePath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return "/"
	}
	if !strings.HasPrefix(path, "/") {
		return "/" + path
	}
	return path
}

func (h *Helper) BuildRequest(ctx context.Context, baseURL string, input ProxyQuery) (*http.Request, error) {
	method := NormalizeMethod(input.Method)
	path := NormalizePath(input.Path)

	targetURL, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid base URL: %w", err)
	}
	targetURL.Path = strings.TrimRight(targetURL.Path, "/") + path

	if len(input.QueryParams) > 0 {
		q := targetURL.Query()
		for key, value := range input.QueryParams {
			q.Set(key, value)
		}
		targetURL.RawQuery = q.Encode()
	}

	var bodyReader io.Reader
	if input.Body != "" {
		bodyReader = strings.NewReader(input.Body)
	}

	// URL is constructed from the configured Perses server base URL and validated datasource proxy path.
	req, err := http.NewRequestWithContext(ctx, method, targetURL.String(), bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	for key, value := range input.Headers {
		if _, blocked := blockedInputHeaders[strings.ToLower(key)]; blocked {
			continue
		}
		req.Header.Set(key, value)
	}
	if input.Body != "" && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	return req, nil
}

func (h *Helper) Execute(req *http.Request) (*QueryResult, error) {
	restClient := h.client.RESTClient()
	if restClient == nil || restClient.Client == nil {
		return nil, fmt.Errorf("perses REST client is not configured")
	}
	if restClient.BaseURL == nil {
		return nil, fmt.Errorf("perses REST client base URL is not configured")
	}
	if req == nil || req.URL == nil {
		return nil, fmt.Errorf("request URL is not configured")
	}

	// Enforce proxy requests against the configured Perses base URL to avoid SSRF.
	if !strings.EqualFold(req.URL.Scheme, restClient.BaseURL.Scheme) || !strings.EqualFold(req.URL.Host, restClient.BaseURL.Host) {
		return nil, fmt.Errorf("request URL must target configured Perses host")
	}

	for key, value := range restClient.Headers {
		if req.Header.Get(key) == "" {
			req.Header.Set(key, value)
		}
	}

	// #nosec G704 -- req URL is built from configured base URL and validated above.
	resp, err := restClient.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	if err != nil {
		return nil, err
	}

	headers := map[string]string{}
	for key, values := range resp.Header {
		if len(values) > 0 {
			headers[key] = strings.Join(values, ", ")
		}
	}

	return &QueryResult{
		StatusCode: resp.StatusCode,
		Headers:    headers,
		Body:       string(body),
	}, nil
}
