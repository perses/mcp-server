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

var (
	rootCmd = &cobra.Command{
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

	stdioCmd = &cobra.Command{
		Use:   "stdio",
		Short: "Start stdio MCP server",
		Long:  "Start a Perses MCP server that communicates via standard input/output streams using JSON-RPC messages.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg := loadConfig("stdio")
			return permcp.Serve(cmd.Context(), cfg)
		},
	}

	httpCmd = &cobra.Command{
		Use:   "http",
		Short: "Start HTTP Streamable MCP server",
		Long:  "Start a Perses MCP server that communicates via HTTP using streamable JSON-RPC messages.",
		RunE: func(cmd *cobra.Command, _ []string) error {

			cfg := loadConfig("http")
			return permcp.Serve(cmd.Context(), cfg)
		},
	}
)

func init() {
	cobra.OnInitialize(
		func() { viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_")) },
		viper.AutomaticEnv,
	)

	// Global flags
	rootCmd.PersistentFlags().String("perses-server-url", "http://localhost:8080", "The Perses backend server URL")
	rootCmd.PersistentFlags().String("log-level", "info", "Log level (debug, info, warn, error)")
	rootCmd.PersistentFlags().String("log-file-path", "", "Path to the log file (if empty, logs go to stderr)")
	rootCmd.PersistentFlags().Bool("read-only", false, "Restrict the server to read-only operations")

	// HTTP Streamable specific flags
	httpCmd.PersistentFlags().String("port", "8000", "Port to run the HTTP Streamable server on")

	// Bind flags to viper
	_ = viper.BindPFlag("perses-server-url", rootCmd.PersistentFlags().Lookup("perses-server-url"))
	_ = viper.BindPFlag("log-level", rootCmd.PersistentFlags().Lookup("log-level"))
	_ = viper.BindPFlag("read-only", rootCmd.PersistentFlags().Lookup("read-only"))
	_ = viper.BindPFlag("log-file-path", rootCmd.PersistentFlags().Lookup("log-file-path"))

	_ = viper.BindPFlag("port", httpCmd.PersistentFlags().Lookup("port"))
	rootCmd.AddCommand(stdioCmd)
	rootCmd.AddCommand(httpCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

func loadConfig(transport string) permcp.Config {
	return permcp.Config{
		Version:         version,
		Transport:       transport,
		PersesServerURL: viper.GetString("perses-server-url"),
		Token:           viper.GetString(envPersesToken),
		ReadOnly:        viper.GetBool("read-only"),
		LogFilePath:     viper.GetString("log-file-path"),
		LogLevel:        viper.GetString("log-level"),
		Port:            viper.GetString("port"),
	}
}
