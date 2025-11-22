package permcp

import "log/slog"

type MCPServerConfig struct {
	// Version of the MCP server
	Version string

	// PersesServerURL is the URL of the Perses backend server
	PersesServerURL string

	// Token is the authentication token for the Perses server
	Token string

	// ReadOnly indicates if the server should operate in read-only mode
	ReadOnly bool

	// Logger for the MCP server
	Logger *slog.Logger

	// LogFilePath is the path to the log file (if empty, logs go to stderr)
	LogFilePath string

	// Transport mechanism for the MCP server (e.g., "stdio", "http-streamable")
	Transport string

	// Port to run the HTTP Streamable server on
	Port string
}

func RunMCPServer(cfg MCPServerConfig) error {
	return nil
}
