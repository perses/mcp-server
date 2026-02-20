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
	"os"
	"strings"

	permcp "github.com/perses/mcp-server/internal/permcp"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	envPersesToken = "PERSES_TOKEN"
)

var version = "version"
var commit = "commit"
var date = "date"

func newRootCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "permcp",
		Short:   "Perses MCP Server",
		Long:    "A Perses MCP Server to manage Perses resources",
		Version: fmt.Sprintf("Version: %s\nCommit: %s\nBuild Date: %s", version, commit, date),
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if viper.GetString(envPersesToken) == "" {
				return fmt.Errorf("environment variable %s is required", envPersesToken)
			}
			return nil
		},
	}

	// Global flags
	cmd.PersistentFlags().String("perses-server-url", "http://localhost:8080", "The Perses backend server URL")
	cmd.PersistentFlags().String("log.level", "info", "Log level (debug, info, warn, error)")
	cmd.PersistentFlags().String("log.file-path", "", "Path to the log file (if empty, logs go to stderr)")
	cmd.PersistentFlags().Bool("read-only", false, "Restrict the server to read-only operations")
	cmd.PersistentFlags().String("resources", "", "Comma-separated list of resources to register (e.g., project,dashboard,globaldatasource). If not specified, all resources are registered.")

	// Bind flags to viper
	_ = viper.BindPFlag("perses-server-url", cmd.PersistentFlags().Lookup("perses-server-url"))
	_ = viper.BindPFlag("log.level", cmd.PersistentFlags().Lookup("log.level"))
	_ = viper.BindPFlag("read-only", cmd.PersistentFlags().Lookup("read-only"))
	_ = viper.BindPFlag("log.file-path", cmd.PersistentFlags().Lookup("log.file-path"))
	_ = viper.BindPFlag("resources", cmd.PersistentFlags().Lookup("resources"))

	// Add subcommands
	cmd.AddCommand(newStdioCommand())
	cmd.AddCommand(newHttpCommand())

	return cmd
}

func newStdioCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "stdio",
		Short: "Start stdio MCP server",
		Long:  "Start a Perses MCP server that communicates via standard input/output streams using JSON-RPC messages.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg := loadConfig("stdio")
			return permcp.Serve(cmd.Context(), cfg)
		},
	}
}

func newHttpCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "http",
		Short: "Start HTTP Streamable MCP server",
		Long:  "Start a Perses MCP server that communicates via HTTP using streamable JSON-RPC messages.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg := loadConfig("http")
			return permcp.Serve(cmd.Context(), cfg)
		},
	}

	// HTTP Streamable specific flags
	cmd.PersistentFlags().String("port", "8000", "Port to run the HTTP Streamable server on")
	_ = viper.BindPFlag("port", cmd.PersistentFlags().Lookup("port"))

	return cmd
}

func main() {
	cobra.OnInitialize(
		func() { viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_", ".", "_")) },
		viper.AutomaticEnv,
	)

	rootCmd := newRootCommand()

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

func loadConfig(transport string) permcp.Config {
	// Parse resources from comma-separated string and normalize to lowercase
	var allowedResources []string
	if resourcesStr := viper.GetString("resources"); resourcesStr != "" {
		for rs := range strings.SplitSeq(resourcesStr, ",") {
			rs = strings.TrimSpace(rs)
			if rs != "" {
				allowedResources = append(allowedResources, strings.ToLower(rs))
			}
		}
	}

	return permcp.Config{
		Version:          version,
		Transport:        transport,
		PersesServerURL:  viper.GetString("perses-server-url"),
		Token:            viper.GetString(envPersesToken),
		ReadOnly:         viper.GetBool("read-only"),
		LogFilePath:      viper.GetString("log.file-path"),
		LogLevel:         viper.GetString("log.level"),
		Port:             viper.GetString("port"),
		AllowedResources: allowedResources,
	}
}
