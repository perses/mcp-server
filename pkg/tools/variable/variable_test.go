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

package variable

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	apiClient "github.com/perses/perses/pkg/client/api"
	"github.com/perses/perses/pkg/client/config"
	v1 "github.com/perses/perses/pkg/model/api/v1"
	"github.com/perses/spec/go/common"
	"github.com/perses/spec/go/dashboard/variable"
)

// newFakePerses starts a Perses API that only accepts the creation of variables in the project "demo".
// The variables received are sent to the returned channel.
func newFakePerses(t *testing.T) (*httptest.Server, <-chan *v1.Variable) {
	created := make(chan *v1.Variable, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/projects/demo/variables" {
			http.NotFound(w, r)
			return
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		entity := &v1.Variable{}
		if err := json.Unmarshal(body, entity); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		created <- entity
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	t.Cleanup(server.Close)
	return server, created
}

func TestCreateProjectVariable(t *testing.T) {
	testSuites := []struct {
		title         string
		arguments     map[string]any
		expectedValue string
	}{
		{
			title:         "value provided",
			arguments:     map[string]any{"project": "demo", "name": "env", "value": "production"},
			expectedValue: "production",
		},
		{
			title:         "value not provided",
			arguments:     map[string]any{"project": "demo", "name": "env"},
			expectedValue: "",
		},
	}
	for _, test := range testSuites {
		t.Run(test.title, func(t *testing.T) {
			ctx := context.Background()
			persesServer, created := newFakePerses(t)
			restClient, err := config.NewRESTClient(config.RestConfigClient{URL: common.MustParseURL(persesServer.URL)})
			if err != nil {
				t.Fatal(err)
			}

			mcpServer := mcp.NewServer(&mcp.Implementation{Name: "test"}, nil)
			New(apiClient.NewWithClient(restClient).V1()).Create().RegisterWith(mcpServer)
			clientTransport, serverTransport := mcp.NewInMemoryTransports()
			if _, err = mcpServer.Connect(ctx, serverTransport, nil); err != nil {
				t.Fatal(err)
			}
			session, err := mcp.NewClient(&mcp.Implementation{Name: "test"}, nil).Connect(ctx, clientTransport, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				_ = session.Close()
			}()

			result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "perses_create_project_variable", Arguments: test.arguments})
			if err != nil {
				t.Fatal(err)
			}
			if result.IsError {
				t.Fatalf("the tool returned an error: %+v", result.Content)
			}

			entity := <-created
			spec, ok := entity.Spec.Spec.(*variable.TextSpec)
			if !ok {
				t.Fatalf("expected a text variable, got %T", entity.Spec.Spec)
			}
			if spec.Value != test.expectedValue {
				t.Errorf("expected the value %q, got %q", test.expectedValue, spec.Value)
			}
		})
	}
}
