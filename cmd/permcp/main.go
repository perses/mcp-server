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
	"flag"

	"github.com/perses/common/app"
	"github.com/sirupsen/logrus"

	permcp "github.com/perses/mcp-server/internal/permcp"
)

func main() {
	configFile := flag.String("config", "", "Path to the YAML configuration file")
	flag.Parse()

	cfg, err := permcp.ResolveConfig(*configFile)
	if err != nil {
		logrus.WithError(err).Fatal("unable to resolve configuration")
	}
	mcpServer, err := permcp.New(cfg)
	if err != nil {
		logrus.WithError(err).Fatal("unable to create mcp server")
	}

	runner := app.NewRunner().
		WithTasks(mcpServer)
	runner.Start()
}
