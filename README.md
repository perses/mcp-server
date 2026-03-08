<div align="center">
<img src="https://raw.githubusercontent.com/perses/perses/main/docs/images/perses_logo_cropped.svg" alt="Perses">
    <h1 align="center">MCP Server for Perses</h1>
</div>

> This MCP Server is currently in **beta**. Features and tools may change, and stability is not guaranteed. Feedback and contributions are most welcome!

## Overview

The Perses MCP Server is a local [Model Context Protocol (MCP)](https://modelcontextprotocol.io/introduction) Server that enables the LLM hosts(Opencode, Claude Desktop, VS Code, Cursor) to interact with the Perses Application in a standardized way.

## Demo

<details open>
<summary> Opencode</summary>
  
https://github.com/user-attachments/assets/87137515-1b45-442d-a4c9-68f460a1ba4c
</details>

<details open>
<summary> Claude Desktop</summary>
  
https://github.com/user-attachments/assets/87137515-1b45-442d-a4c9-68f460a1ba4c
</details>

<details>
<summary>VS Code with GitHub Copilot</summary>

https://github.com/user-attachments/assets/b80c354a-8006-4e1f-b7f4-e123002f7dc3

</details>

## Pre-requisites

- A running [Perses](https://github.com/perses/perses) instance
- A configuration file for the MCP server (see [Configuration File](#configuration-file))

### Authentication

There are two main ways to authenticate the MCP server with your Perses instance.
1. Basic Auth using your Perses username and password
2. Bearer token obtained via the Perses CLI `percli` command-line tool.

#### Basic Authentication (Username/Password)

Use your Perses username and password directly:

```yaml
perses_server:
  url: "https://perses.example.com"
  native_auth:
    login: "your-username"
    password: "your-password"
```

> **Tip**: You can store sensitive values like passwords as environment variables instead of in the config file (e.g., `PERMCP_PERSES_SERVER_NATIVE_AUTH_PASSWORD`). See [Environment Variables](#environment-variables) for details.

#### Bearer Token (via `percli`)

1. Install [percli](https://perses.dev/perses/docs/cli/) and login to your Perses server:

```bash
percli login <PERSES_SERVER_URL>
```

For example, `percli login https://demo.perses.dev`

2. After successful login, retrieve your token:

```bash
percli whoami --show-token
```

3. Use the token in your configuration file:

```yaml
perses_server:
  url: "https://perses.example.com"
  authorization:
    type: Bearer
    credentials: "<YOUR_TOKEN>"
```

> **Tip**: You can also store the token as an environment variable `PERMCP_PERSES_SERVER_AUTHORIZATION_CREDENTIALS` instead of in the config file. See [Environment Variables](#environment-variables) for details.

> **WARNING**: The bearer token automatically expires based on the `access_token_ttl` setting (default: 15 minutes) of the Perses server. You can change this in the Perses app [configuration](https://perses.dev/perses/docs/configuration/configuration/?h=configu).

## Download the MCP Server Binary

**Download from Releases**

1. Go to the [releases page](https://github.com/perses/mcp-server/releases)
2. Download the appropriate binary for your operating system and architecture
3. Extract the binary to a location of your choice
4. Make the binary executable (on Unix-like systems):
   ```bash
   chmod +x /path/to/mcp-server
   ```
5. Copy the absolute path to the binary (e.g., /path/to/mcp-server) to use in your MCP server configuration

## Transport Modes

The Perses MCP Server supports both the transport modes: STDIO mode and Streamable HTTP mode.

### STDIO Mode

In this mode, the MCP server communicates with the LLM host via standard input and output (STDIO). 

For more details, see the [MCP Protocol Specification docs](https://modelcontextprotocol.io/specification/2025-06-18/basic/transports#stdio).

<details>
<summary>Install in Claude Desktop</summary>

https://github.com/user-attachments/assets/b80c354a-8006-4e1f-b7f4-e123002f7dc3

To add this MCP server to [Claude Desktop](https://claude.ai/download):

1. Create a configuration file (e.g., `perses-mcp-config.yaml`):

```yaml
transport: stdio
perses_server:
  url: "http://localhost:8080"
  native_auth:
    login: "admin"
    password: "password"
```

2. Create or edit the Claude Desktop configuration file at:

   - macOS: `~/Library/Application Support/Claude/claude_desktop_config.json`
   - Windows: `%APPDATA%\Claude\claude_desktop_config.json`
   - Linux: `~/.config/Claude/claude_desktop_config.json`

You can easily access this file via the Claude Desktop app by navigating to `Claude > Settings > Developer > Edit Config`.

3. Add the following JSON block to the configuration file:

```json
{
  "mcpServers": {
    "perses-mcp": {
      "command": "<ABSOLUTE_PATH_TO_PERSES_MCP_BINARY>",
      "args": [
        "--config",
        "<ABSOLUTE_PATH_TO_CONFIG_YAML>"
      ],
      "env": {
        "PERMCP_PERSES_SERVER_NATIVE_AUTH_PASSWORD": "<YOUR_PASSWORD>"
      }
    }
  }
}
```

> **Tip**: Sensitive values like passwords and tokens should be stored as environment variables in the `"env"` block rather than in the config file.

4. Restart Claude Desktop for the changes to take effect.
</details>

<details>
<summary>Install in VS Code</summary>

Add the following JSON code snippet to the VS Code MCP Config file. See [VS Code MCP documentation](https://code.visualstudio.com/docs/copilot/chat/mcp-servers) for more details.

```json
{
  "servers": {
    "perses-mcp": {
      "command": "<ABSOLUTE_PATH_TO_PERSES_MCP_BINARY>",
      "args": [
        "--config",
        "<ABSOLUTE_PATH_TO_CONFIG_YAML>"
      ],
      "env": {
        "PERMCP_PERSES_SERVER_NATIVE_AUTH_PASSWORD": "<YOUR_PASSWORD>"
      }
    }
  }
}
```
</details>

<details>
<summary>Install in OpenCode</summary>

Add the following to your [OpenCode Config](https://opencode.ai/docs/config/) under `mcp`. See the [OpenCode MCP documentation](https://opencode.ai/docs/mcp-servers/) for more details.

```json
{
  "mcp": {
    "perses": {
      "command": [
        "<ABSOLUTE_PATH_TO_PERSES_MCP_BINARY>",
        "--config",
        "<ABSOLUTE_PATH_TO_CONFIG_YAML>"
      ],
      "environment": {
        "PERMCP_PERSES_SERVER_NATIVE_AUTH_LOGIN": "{env:PERMCP_PERSES_SERVER_NATIVE_AUTH_LOGIN}",
        "PERMCP_PERSES_SERVER_NATIVE_AUTH_PASSWORD": "{env:PERMCP_PERSES_SERVER_NATIVE_AUTH_PASSWORD}"
      },
      "enabled": true,
      "type": "local"
    }
  }
}
```
</details>


### Streamable HTTP Mode
The Streamable HTTP mode allows the MCP server to communicate with LLM hosts over HTTP, similar to a regular web API. This mode is particularly useful for:

- **Remote hosting**: Deploy the MCP server on a cloud instance or remote server
- **Multiple clients**: Allow multiple LLM hosts to connect to the same server instance

For more details, see the [MCP Protocol Specification Docs](https://modelcontextprotocol.io/specification/2025-06-18/basic/transports#streamable-http).

Create a configuration file (e.g., `config.yaml`) with the HTTP transport:

```yaml
transport: http
listen_address: ":8000"
perses_server:
  url: "http://localhost:8080"
  native_auth:
    login: "admin"
    password: "password"
```

<details>
<summary>Install in VS Code</summary>
Add the following JSON code snippet to the VS Code MCP Config file. See [VS Code MCP documentation](https://code.visualstudio.com/docs/copilot/chat/mcp-servers) for more details.

```json
{
  "servers": {
    "perses-http": {
      "type": "http",
      "url": "http://localhost:<port>/mcp"
    }
  }
}
```
</details>

## Command-Line Usage

```bash
perses-mcp-server --config /path/to/config.yaml
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--config` | `""` | Path to the YAML configuration file |
| `-log.level` | `info` | Log level (options: `panic`, `fatal`, `error`, `warning`, `info`, `debug`, `trace`) |
| `-log.format` | `text` | Log format (options: `text`, `json`) |
| `-log.method-trace` | `false` | Include the calling method as a field in the log |

### Configuration File

The MCP server is configured primarily through a YAML configuration file passed via the `--config` flag.

```yaml
# MCP transport mode: "stdio" or "http"
transport: stdio

# Address to listen on for HTTP transport (e.g., ":8000")
listen_address: ":8000"

# Restrict the server to read-only operations
read_only: false

# Comma-separated list of resources to register (if empty, all resources are registered)
resources: ""

# Perses server connection configuration
perses_server:
  url: "http://localhost:8080"

  # Authentication (choose one method):

  # Option 1: Native authentication (login/password)
  # native_auth:
  #   login: "admin"
  #   password: "password"

  # Option 2: Bearer token (e.g., from `percli whoami --show-token`)
  # authorization:
  #   type: Bearer
  #   credentials: "<YOUR_TOKEN>"
  #   # credentials_file: "/path/to/token/file"  # Alternative: read token from file

  # TLS configuration (optional)
  # tls_config:
  #   ca_file: "/path/to/ca.pem"
  #   insecure_skip_verify: false
```

#### Available Resources

The `resources` field accepts the following resource names (case-insensitive, comma-separated):

| Resource | Description |
|---------|-------------|
| `dashboard` | Dashboard management tools |
| `project` | Project management tools |
| `datasource` | Project-level datasource tools |
| `globaldatasource` | Global datasource tools |
| `role` | Project-level role tools |
| `globalrole` | Global role tools |
| `rolebinding` | Project-level role binding tools |
| `globalrolebinding` | Global role binding tools |
| `variable` | Project-level variable tools |
| `globalvariable` | Global variable tools |
| `plugin` | Plugin tools |

### Environment Variables

Configuration values in the YAML file can be overridden using environment variables with the `PERMCP_` prefix. The variable name is derived by uppercasing each YAML key and joining nested keys with `_`.

For example, the YAML path `perses_server.native_auth.password` becomes:

```
PERMCP_PERSES_SERVER_NATIVE_AUTH_PASSWORD
```

This is particularly useful for sensitive values like passwords and tokens that should not be stored in the config file.

| Environment Variable | Config Path | Description |
|---------------------|-------------|-------------|
| `PERMCP_TRANSPORT` | `transport` | Transport mode |
| `PERMCP_LISTEN_ADDRESS` | `listen_address` | HTTP listen address |
| `PERMCP_READ_ONLY` | `read_only` | Read-only mode |
| `PERMCP_RESOURCES` | `resources` | Resources to register |
| `PERMCP_PERSES_SERVER_URL` | `perses_server.url` | Perses server URL |
| `PERMCP_PERSES_SERVER_NATIVE_AUTH_LOGIN` | `perses_server.native_auth.login` | Native auth username |
| `PERMCP_PERSES_SERVER_NATIVE_AUTH_PASSWORD` | `perses_server.native_auth.password` | Native auth password |
| `PERMCP_PERSES_SERVER_AUTHORIZATION_TYPE` | `perses_server.authorization.type` | Authorization type (e.g., `Bearer`) |
| `PERMCP_PERSES_SERVER_AUTHORIZATION_CREDENTIALS` | `perses_server.authorization.credentials` | Authorization token |

For more details about how environment variables override the configuration file, see the [Perses Configuration docs](https://perses.dev/perses/docs/configuration/configuration/?h=perses_#configuration-file).

### Configuration Precedence

Configuration values are resolved in this order (later wins):

1. Built-in defaults
2. YAML configuration file (provided via `--config`)
3. Environment variables with `PERMCP_` prefix

## Local Development

### Build from Source

If you want to build the MCP server from source code (for development or contribution purposes), run the following command from the source code root directory:

```bash
make build
```

This should create a `bin` directory which contains the binary named `mcp-server`. Copy the absolute path to the binary to use in your MCP server configuration.

## Tools

> [!NOTE]  
> When running in read-only mode (`read_only: true` in config), only tools that retrieve information are available. Write operations like `create_project`, `create_dashboard`, `create_global_datasource`, `update_global_datasource`, and `create_project_variable` are disabled in read-only mode.

### Projects

| Tool                         | Description           | Required Parameters |
| ---------------------------- | --------------------- | ------------------- |
| `perses_list_projects`       | List all projects     | -                   |
| `perses_get_project_by_name` | Get a project by name | `project`           |
| `perses_create_project`      | Create a new project  | `project`           |

### Dashboards

| Tool                           | Description                                                    | Required Parameters    |
| ------------------------------ | -------------------------------------------------------------- | ---------------------- |
| `perses_list_dashboards`       | List all dashboards for a specific project                     | `project`              |
| `perses_get_dashboard_by_name` | Get a dashboard by name for a project                          | `project`, `dashboard` |
| `perses_create_dashboard`      | Create a dashboard given a project and dashboard configuration | `project`, `dashboard` |

For dashboard configuration, see [Perses Dashboards](https://github.com/perses/perses/blob/main/docs/api/dashboard.md)

### Datasources

| Tool                                    | Description                                 | Required Parameters     | Optional Parameters          |
| --------------------------------------- | ------------------------------------------- | ----------------------- | ---------------------------- |
| `perses_list_global_datasources`        | List all global datasources                 | -                       | -                            |
| `perses_list_datasources`               | List all datasources for a specific project | `project`               | -                            |
| `perses_get_global_datasource_by_name`  | Get a global datasource by name             | `datasource`            | -                            |
| `perses_get_project_datasource_by_name` | Get a project datasource by name            | `project`, `datasource` | -                            |
| `perses_create_global_datasource`       | Create a new global datasource              | `name`, `type`, `url`   | `display_name`, `proxy_type` |
| `perses_update_global_datasource`       | Update an existing global datasource        | `name`, `type`, `url`   | `display_name`, `proxy_type` |

### Roles

| Tool                                      | Description                           | Required Parameters      |
| ----------------------------------------- | ------------------------------------- | ------------------------ |
| `perses_list_global_roles`                | List all global roles                 | -                        |
| `perses_get_global_role_by_name`          | Get a global role by name             | `role`                   |
| `perses_list_global_role_bindings`        | List all global role bindings         | -                        |
| `perses_get_global_role_binding_by_name`  | Get a global role binding by name     | `roleBinding`            |
| `perses_list_project_roles`               | List all roles for a specific project | `project`                |
| `perses_get_project_role_by_name`         | Get a project role by name            | `project`, `role`        |
| `perses_list_project_role_bindings`       | List all role bindings for a project  | `project`                |
| `perses_get_project_role_binding_by_name` | Get a project role binding by name    | `project`, `roleBinding` |

### Plugins

| Tool                  | Description      | Required Parameters |
| --------------------- | ---------------- | ------------------- |
| `perses_list_plugins` | List all plugins | -                   |

### Variables

| Tool                                  | Description                               | Required Parameters   |
| ------------------------------------- | ----------------------------------------- | --------------------- |
| `perses_list_global_variables`        | List all global variables                 | -                     |
| `perses_get_global_variable_by_name`  | Get a global variable by name             | `variable`            |
| `perses_list_variables`               | List all variables for a specific project | `project`             |
| `perses_get_project_variable_by_name` | Get a project variable by name            | `project`, `variable` |
| `perses_create_project_variable`      | Create a project level variable           | `name`, `project`     |

## License

The code is licensed under an [Apache 2.0](./LICENSE) license.
