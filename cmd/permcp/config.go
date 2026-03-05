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

package main

import (
	"fmt"

	commonconfig "github.com/perses/common/config"

	permcp "github.com/perses/mcp-server/internal/permcp"
)

func resolveConfig(configFile string) (permcp.Config, error) {
	resolved := permcp.Config{}
	if err := commonconfig.NewResolver[permcp.Config]().SetEnvPrefix("PERMCP").SetConfigFile(configFile).Resolve(&resolved).Verify(); err != nil {
		return permcp.Config{}, fmt.Errorf("unable to resolve configuration: %w", err)
	}

	return resolved, nil
}
