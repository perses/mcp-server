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
	"github.com/perses/mcp-server/pkg/tools"
	"github.com/perses/mcp-server/pkg/tools/resource"
	apiclient "github.com/perses/perses/pkg/client/api"
)

type validation struct {
	client apiclient.ClientInterface
}

func New(client apiclient.ClientInterface) resource.Resource {
	return &validation{client: client}
}

func (v *validation) GetTools() []*tools.Tool {
	return []*tools.Tool{v.Dashboard(), v.Variable()}
}

func (v *validation) Create() *tools.Tool { return nil }
func (v *validation) Update() *tools.Tool { return nil }
func (v *validation) Delete() *tools.Tool { return nil }
func (v *validation) List() *tools.Tool   { return nil }
func (v *validation) Get() *tools.Tool    { return nil }
