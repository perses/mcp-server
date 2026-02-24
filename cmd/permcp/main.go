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
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	permcp "github.com/perses/mcp-server/internal/permcp"
)

const (
	envPersesToken = "PERSES_TOKEN"
)

var version = "version"

func main() {
	var showVersion bool
	flag.BoolVar(&showVersion, "version", false, "Print version information and exit")

	inputs := commandInput{}
	flag.StringVar(&inputs.ConfigFile, "config", "", "Path to the YAML configuration file")
	flag.StringVar(&inputs.Transport, "transport", "", "MCP transport (stdio or http)")
	flag.StringVar(&inputs.PersesServerURL, "perses-server-url", "", "The Perses backend server URL")
	flag.StringVar(&inputs.Port, "port", "", "Port to run the HTTP Streamable server on")
	flag.BoolVar(&inputs.ReadOnly, "read-only", false, "Restrict the server to read-only operations")
	flag.StringVar(&inputs.Resources, "resources", "", "Comma-separated list of resources to register (e.g., project,dashboard,globaldatasource). If not specified, all resources are registered.")
	flag.StringVar(&inputs.LogFilePath, "log.file-path", "", "Path to the log file (if empty, logs go to stderr)")
	flag.String("log.level", "info", "Log level (debug, info, warn, error)")

	flag.Parse()

	cfg, err := resolveConfig(inputs)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if err := permcp.Serve(ctx, cfg); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}
