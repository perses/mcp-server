package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/perses/mcp-server/internal/permcp"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const PERSES_TOKEN = "PERSES_TOKEN"

var version = "version"
var commit = "commit"
var date = "date"

var (
	rootCmd = &cobra.Command{
		Use:     "permcp",
		Short:   "Perses MCP Server",
		Long:    "A Perses MCP Server to manage Perses resources",
		Version: fmt.Sprintf("Version: %s\nCommit: %s\nBuild Date: %s", version, commit, date),
		RunE: func(_ *cobra.Command, _ []string) error {

			token := viper.GetString(PERSES_TOKEN)
			if token == "" {
				return fmt.Errorf("%s not set", PERSES_TOKEN)
			}

			mcpServerConfig := permcp.MCPServerConfig{
				Version:         version,
				PersesServerURL: viper.GetString("perses-server-url"),
				Token:           token,
				ReadOnly:        viper.GetBool("read-only"),
				Transport:       viper.GetString("transport"),
				Port:            viper.GetString("port"),
				LogFilePath:     viper.GetString("log-file-path"),
			}
			return permcp.RunMCPServer(mcpServerConfig)
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
	rootCmd.PersistentFlags().String("transport", "stdio", "MCP protocol currently supports 'stdio' and 'http-streamable' transport mechanisms")
	rootCmd.PersistentFlags().String("port", "8000", "Port to run the HTTP Streamable server on")
	rootCmd.PersistentFlags().Bool("read-only", false, "Restrict the server to read-only operations")

	// Bind flags to viper
	viper.BindPFlag("perses-server-url", rootCmd.PersistentFlags().Lookup("perses-server-url"))
	viper.BindPFlag("log-level", rootCmd.PersistentFlags().Lookup("log-level"))
	viper.BindPFlag("transport", rootCmd.PersistentFlags().Lookup("transport"))
	viper.BindPFlag("port", rootCmd.PersistentFlags().Lookup("port"))
	viper.BindPFlag("read-only", rootCmd.PersistentFlags().Lookup("read-only"))
	viper.BindPFlag("log-file-path", rootCmd.PersistentFlags().Lookup("log-file-path"))

}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}
