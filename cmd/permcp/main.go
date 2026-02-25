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

	"github.com/prometheus/common/version"

	permcp "github.com/perses/mcp-server/internal/permcp"
)

const (
	envPersesToken = "PERSES_TOKEN"
)

func main() {
	inputs := commandInput{}
	var showVersion bool
	flag.StringVar(&inputs.ConfigFile, "config", "", "Path to the YAML configuration file")
	flag.BoolVar(&showVersion, "version", false, "Print binary version and exit")

	flag.Parse()

	if showVersion {
		if version.Version == "" {
			fmt.Println("dev")
			return
		}

		fmt.Println(version.Version)
		return
	}

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
