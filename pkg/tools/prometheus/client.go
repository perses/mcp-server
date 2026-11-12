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
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/perses/mcp-server/pkg/tools/internal/proxy"
	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
)

const (
	// maxBodyBytes caps the size of the Prometheus responses, like the datasource query tools.
	maxBodyBytes = 1 << 20 // 1 MiB
	// maxErrorLength caps the error messages returned by Perses or the datasource.
	maxErrorLength = 400
)

var errTooMuchData = errors.New("the query returns too much data (over 1 MiB): aggregate it (sum by, topk) or narrow the label matchers")

// newAPI returns a client of the Prometheus HTTP API that sends its requests through the Perses proxy of the
// datasource, with the HTTP client and the default headers (credentials) of the Perses client.
func (p *prometheus) newAPI(datasource DatasourceRef) (v1.API, error) {
	restClient := p.client.RESTClient()
	if restClient == nil || restClient.Client == nil || restClient.BaseURL == nil {
		return nil, errors.New("perses REST client is not configured")
	}
	path := proxy.GlobalDatasourceProxyPath(datasource.Name, "")
	if datasource.Project != "" {
		path = proxy.ProjectDatasourceProxyPath(datasource.Project, datasource.Name, "")
	}
	client, err := api.NewClient(api.Config{
		Address: strings.TrimRight(restClient.BaseURL.String(), "/") + path,
		Client:  &http.Client{Transport: &persesTransport{client: restClient.Client, headers: restClient.Headers}},
	})
	if err != nil {
		return nil, fmt.Errorf("error creating the Prometheus client of %s: %w", datasource, err)
	}
	return v1.NewAPI(client), nil
}

// persesTransport sends the requests with the Perses client and caps the size of the response bodies.
//
// The Prometheus client only keeps the body of the errors in the Prometheus format: the other error responses (e.g.
// a Perses error or the error page of a proxy) are turned into a statusError that keeps their message. The responses
// with which the Prometheus client retries a POST request with GET (403, 405 and 501) are passed through.
type persesTransport struct {
	client  *http.Client
	headers map[string]string
}

func (t *persesTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	for key, value := range t.headers {
		if req.Header.Get(key) == "" {
			req.Header.Set(key, value)
		}
	}
	// #nosec G704 -- the URL is built from the configured Perses base URL and the datasource proxy path.
	resp, err := t.client.Do(req)
	if err != nil {
		return nil, err
	}
	resp.Body = &limitedBody{ReadCloser: resp.Body, remaining: maxBodyBytes}
	if resp.StatusCode/100 == 2 || (req.Method == http.MethodPost && isGetFallbackStatus(resp.StatusCode)) {
		return resp, nil
	}
	body, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		return nil, err
	}
	if _, isPrometheusError := prometheusErrorMessage(body); isPrometheusError {
		resp.Body = io.NopCloser(bytes.NewReader(body))
		return resp, nil
	}
	return nil, &statusError{status: resp.Status, message: errorMessage(body)}
}

// isGetFallbackStatus reports whether the Prometheus client retries a POST request with GET after this status.
func isGetFallbackStatus(status int) bool {
	return status == http.StatusForbidden || status == http.StatusMethodNotAllowed || status == http.StatusNotImplemented
}

// limitedBody fails with errTooMuchData when the body is larger than its limit.
type limitedBody struct {
	io.ReadCloser
	remaining int64
}

func (b *limitedBody) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	b.remaining -= int64(n)
	if b.remaining < 0 {
		return n, errTooMuchData
	}
	return n, err
}

// statusError is an HTTP error response of Perses or of the datasource that is not a Prometheus error.
type statusError struct {
	status  string
	message string
}

func (e *statusError) Error() string {
	if e.message == "" {
		return "HTTP " + e.status
	}
	return fmt.Sprintf("HTTP %s: %s", e.status, e.message)
}

// prometheusErrorMessage returns "<errorType>: <error>" when body is an error of the Prometheus HTTP API.
func prometheusErrorMessage(body []byte) (string, bool) {
	var response struct {
		Status    string `json:"status"`
		ErrorType string `json:"errorType"`
		Error     string `json:"error"`
	}
	if json.Unmarshal(body, &response) != nil || response.Status != "error" {
		return "", false
	}
	return fmt.Sprintf("%s: %s", response.ErrorType, response.Error), true
}

// errorMessage returns the message of an error body: the message of a Perses error, else the body, shortened.
func errorMessage(body []byte) string {
	var persesError struct {
		Message string `json:"message"`
	}
	message := strings.TrimSpace(string(body))
	if json.Unmarshal(body, &persesError) == nil && persesError.Message != "" {
		message = persesError.Message
	}
	if len(message) > maxErrorLength {
		message = message[:maxErrorLength] + "..."
	}
	return message
}

// apiError makes the errors of the Prometheus client readable. Prometheus errors are returned unchanged
// ("bad_data: parse error..."), the other HTTP errors with their status and message.
func apiError(datasource DatasourceRef, err error) error {
	if errors.Is(err, errTooMuchData) {
		return errTooMuchData
	}
	var promErr *v1.Error
	if errors.As(err, &promErr) {
		if promErr.Detail == "" {
			return promErr
		}
		// The Prometheus client keeps the body of the errors it doesn't decode, e.g. a 503 Prometheus error.
		if message, isPrometheusError := prometheusErrorMessage([]byte(promErr.Detail)); isPrometheusError {
			return errors.New(message)
		}
		return fmt.Errorf("error querying %s: %s: %s", datasource, promErr.Msg, errorMessage([]byte(promErr.Detail)))
	}
	// The HTTP client wraps the errors of the transport with the request URL: keep the status and message only.
	var statusErr *statusError
	if errors.As(err, &statusErr) {
		return fmt.Errorf("error querying %s: %w", datasource, statusErr)
	}
	return fmt.Errorf("error querying %s: %w", datasource, err)
}
