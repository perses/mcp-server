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

package globaldatasource

import (
	"testing"

	"github.com/perses/mcp-server/pkg/tools"
	"github.com/perses/mcp-server/pkg/tools/internal/proxytest"
	apiClient "github.com/perses/perses/pkg/client/api/v1"
)

func TestQueryResponse(t *testing.T) {
	proxytest.RunQueryResponseTests(t, proxytest.QueryTool{
		Tools:            func(client apiClient.ClientInterface) []*tools.Tool { return New(client).GetTools() },
		Name:             "perses_query_global_datasource",
		Arguments:        map[string]any{"datasource_name": "prom", "path": "/api/v1/query"},
		WantPath:         "/proxy/globaldatasources/prom/api/v1/query",
		WantErrorContext: "global datasource 'prom'",
	})
}
