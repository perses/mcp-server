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
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	apiclient "github.com/perses/perses/pkg/client/api"
	"github.com/perses/perses/pkg/client/perseshttp"
	"github.com/perses/spec/go/common"
)

const wantMarker = "\n[response truncated: exceeded 1 MiB limit]"

func TestResponseText(t *testing.T) {
	tests := []struct {
		name      string
		response  Response
		want      string
		wantError []string
	}{
		{
			name:     "2xx body is returned unchanged",
			response: Response{StatusCode: http.StatusOK, Body: []byte(" {\"status\":\"success\"}\n")},
			want:     " {\"status\":\"success\"}\n",
		},
		{
			name:     "204 with an empty body returns an empty text",
			response: Response{StatusCode: http.StatusNoContent},
			want:     "",
		},
		{
			name:     "truncated 2xx body gets the marker",
			response: Response{StatusCode: http.StatusOK, Body: []byte(`{"data":[1,2`), Truncated: true},
			want:     `{"data":[1,2` + wantMarker,
		},
		{
			name:      "4xx is an error with the status and the body",
			response:  Response{StatusCode: http.StatusBadRequest, Body: []byte("{\"error\":\"bad_data\"}\n")},
			wantError: []string{"HTTP 400 Bad Request", `{"error":"bad_data"}`},
		},
		{
			name:      "5xx is an error with the status and the body",
			response:  Response{StatusCode: http.StatusBadGateway, Body: []byte("upstream down")},
			wantError: []string{"HTTP 502 Bad Gateway", "upstream down"},
		},
		{
			name:      "truncated error body gets the marker",
			response:  Response{StatusCode: http.StatusInternalServerError, Body: []byte("boom"), Truncated: true},
			wantError: []string{"HTTP 500 Internal Server Error", "boom" + wantMarker},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.response.Text()
			if len(tt.wantError) > 0 {
				if err == nil {
					t.Fatalf("Text() error = nil, want an error")
				}
				for _, want := range tt.wantError {
					if !strings.Contains(err.Error(), want) {
						t.Errorf("error %q does not contain %q", err.Error(), want)
					}
				}
				return
			}
			if err != nil {
				t.Fatalf("Text() unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("Text() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDoTruncation(t *testing.T) {
	tests := []struct {
		name          string
		bodySize      int
		wantTruncated bool
	}{
		{name: "body at the limit is kept", bodySize: maxBodyBytes, wantTruncated: false},
		{name: "body over the limit is truncated", bodySize: maxBodyBytes + 1024, wantTruncated: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(strings.Repeat("a", tt.bodySize)))
			}))
			defer httpServer.Close()
			client := apiclient.NewWithClient(&perseshttp.RESTClient{
				BaseURL: common.MustParseURL(httpServer.URL),
				Client:  httpServer.Client(),
			})

			resp, err := New(client.V1()).Do(context.Background(), ProxyQuery{Path: GlobalDatasourceProxyPath("prom", "/api/v1/query")})
			if err != nil {
				t.Fatalf("Do() unexpected error: %v", err)
			}
			if resp.Truncated != tt.wantTruncated {
				t.Errorf("Truncated = %v, want %v", resp.Truncated, tt.wantTruncated)
			}
			if len(resp.Body) != min(tt.bodySize, maxBodyBytes) {
				t.Errorf("len(Body) = %d, want %d", len(resp.Body), min(tt.bodySize, maxBodyBytes))
			}
			text, err := resp.Text()
			if err != nil {
				t.Fatalf("Text() unexpected error: %v", err)
			}
			if got := strings.HasSuffix(text, wantMarker); got != tt.wantTruncated {
				t.Errorf("text has the truncation marker = %v, want %v", got, tt.wantTruncated)
			}
		})
	}
}
