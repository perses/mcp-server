<div align="center">
<img src="https://raw.githubusercontent.com/perses/perses/main/docs/images/perses_logo_cropped.svg" alt="Perses">
    <h1 align="center">MCP Server for Perses</h1>
</div>

> This MCP Server is currently in **beta**. Features and tools may change, and stability is not guaranteed. Feedback and contributions are most welcome!

## Overview

The Perses MCP Server is a local [Model Context Protocol (MCP)](https://modelcontextprotocol.io/introduction) Server that enables the LLM hosts (OpenCode, Claude Desktop, VS Code, Cursor) to interact with the Perses Application in a standardized way.

## Demo

<details>
<summary>OpenCode</summary>

https://github.com/user-attachments/assets/87137515-1b45-442d-a4c9-68f460a1ba4c
</details>

<details>
<summary>Claude Desktop</summary>

https://github.com/user-attachments/assets/87137515-1b45-442d-a4c9-68f460a1ba4c
</details>

<details>
<summary>VS Code with GitHub Copilot</summary>

https://github.com/user-attachments/assets/b80c354a-8006-4e1f-b7f4-e123002f7dc3
</details>

## Getting Started

### Prerequisites

- A running [Perses](https://github.com/perses/perses) instance
- The MCP server binary — download from the [releases page](https://github.com/perses/mcp-server/releases), extract it, and make it executable:
  ```bash
  chmod +x /path/to/perses-mcp-server
  ```

### 1. Create a configuration file

Create a YAML configuration file (e.g., `perses-mcp-config.yaml`). See [Authentication](#authentication) for details on each auth method.

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

  # Option 1: Basic authentication (login/password)
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

> **Note**: Configuration values are resolved in this order (later wins): built-in defaults < YAML configuration file < environment variables.

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

#### Environment Variables

Configuration values in the YAML file can be overridden using environment variables with the `PERMCP_` prefix. The variable name is derived by uppercasing each YAML key and joining nested keys with `_`.

For example, the YAML path `perses_server.native_auth.password` becomes `PERMCP_PERSES_SERVER_NATIVE_AUTH_PASSWORD`.

This is particularly useful for sensitive values like passwords and tokens that should not be stored in the config file.

| Environment Variable | Config Path | Description |
|---------------------|-------------|-------------|
| `PERMCP_TRANSPORT` | `transport` | Transport mode |
| `PERMCP_LISTEN_ADDRESS` | `listen_address` | HTTP listen address |
| `PERMCP_READ_ONLY` | `read_only` | Read-only mode |
| `PERMCP_RESOURCES` | `resources` | Resources to register |
| `PERMCP_PERSES_SERVER_URL` | `perses_server.url` | Perses server URL |
| `PERMCP_PERSES_SERVER_NATIVE_AUTH_LOGIN` | `perses_server.native_auth.login` | Basic auth username |
| `PERMCP_PERSES_SERVER_NATIVE_AUTH_PASSWORD` | `perses_server.native_auth.password` | Basic auth password |
| `PERMCP_PERSES_SERVER_AUTHORIZATION_TYPE` | `perses_server.authorization.type` | Authorization type (e.g., `Bearer`) |
| `PERMCP_PERSES_SERVER_AUTHORIZATION_CREDENTIALS` | `perses_server.authorization.credentials` | Authorization token |

For more details about how environment variables override the configuration file, see the [Perses Configuration docs](https://perses.dev/perses/docs/configuration/configuration/?h=perses_#configuration-file).

### 2. Add the MCP server to your client

**Standard config** works in most clients:

```json
{
  "mcpServers": {
    "perses-mcp": {
      "command": "<ABSOLUTE_PATH_TO_PERSES_MCP_BINARY>",
      "args": [
        "--config",
        "<ABSOLUTE_PATH_TO_CONFIG_YAML>"
      ]
    }
  }
}
```

> **Tip**: Pass sensitive auth values as environment variables instead of storing them in the config file:
>
> **Basic auth**: `PERMCP_PERSES_SERVER_NATIVE_AUTH_LOGIN` and `PERMCP_PERSES_SERVER_NATIVE_AUTH_PASSWORD`
>
> **Bearer token**: `PERMCP_PERSES_SERVER_AUTHORIZATION_CREDENTIALS`
>
> See [Environment Variables](#environment-variables) for the full list.

<details>
<summary>Claude Desktop</summary>

Create or edit the Claude Desktop configuration file at:

- macOS: `~/Library/Application Support/Claude/claude_desktop_config.json`
- Windows: `%APPDATA%\Claude\claude_desktop_config.json`
- Linux: `~/.config/Claude/claude_desktop_config.json`

You can also access this file via `Claude > Settings > Developer > Edit Config`.

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
        "PERMCP_PERSES_SERVER_NATIVE_AUTH_LOGIN": "<YOUR_LOGIN>",
        "PERMCP_PERSES_SERVER_NATIVE_AUTH_PASSWORD": "<YOUR_PASSWORD>"
      }
    }
  }
}
```

Restart Claude Desktop for the changes to take effect.
</details>

<details>
<summary>VS Code</summary>

Add the following to your VS Code MCP config file. See [VS Code MCP documentation](https://code.visualstudio.com/docs/copilot/chat/mcp-servers) for more details.

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
        "PERMCP_PERSES_SERVER_NATIVE_AUTH_LOGIN": "<YOUR_LOGIN>",
        "PERMCP_PERSES_SERVER_NATIVE_AUTH_PASSWORD": "<YOUR_PASSWORD>"
      }
    }
  }
}
```
</details>

<details>
<summary>OpenCode</summary>

Add the following to your [OpenCode config](https://opencode.ai/docs/config/) under `mcp`. See [OpenCode MCP documentation](https://opencode.ai/docs/mcp-servers/) for more details.

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

## Streamable HTTP Mode

The Streamable HTTP mode allows the MCP server to communicate with LLM hosts over HTTP. This is useful for remote hosting or allowing multiple clients to connect to the same server instance.

For more details, see the [MCP Protocol Specification docs](https://modelcontextprotocol.io/specification/2025-06-18/basic/transports#streamable-http).

Set `transport: http` in your configuration file:

```yaml
transport: http
listen_address: ":8000"
perses_server:
  url: "http://localhost:8080"
  native_auth:
    login: "admin"
    password: "password"
```

Then point your MCP client to the HTTP endpoint:

<details>
<summary>VS Code</summary>

```json
{
  "servers": {
    "perses-http": {
      "type": "http",
      "url": "http://localhost:8000/mcp"
    }
  }
}
```
</details>

## Authentication

There are two ways to authenticate the MCP server with your Perses instance. Add the relevant block under `perses_server` in your [configuration file](#1-create-a-configuration-file).

### Basic Authentication (Username/Password)

Use your Perses username and password directly:

```yaml
native_auth:
  login: "your-username"
  password: "your-password"
```

### Bearer Token (via `percli`)

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
authorization:
  type: Bearer
  credentials: "<YOUR_TOKEN>"
```

> **Warning**: The bearer token automatically expires based on the `access_token_ttl` setting (default: 15 minutes) of the Perses server. You can change this in the Perses app [configuration](https://perses.dev/perses/docs/configuration/configuration/?h=configu).

## Command-Line Usage

```bash
perses-mcp-server --config /path/to/config.yaml
```

| Flag | Default | Description |
|------|---------|-------------|
| `--config` | `""` | Path to the YAML configuration file |
| `-log.level` | `info` | Log level (options: `panic`, `fatal`, `error`, `warning`, `info`, `debug`, `trace`) |
| `-log.format` | `text` | Log format (options: `text`, `json`) |
| `-log.method-trace` | `false` | Include the calling method as a field in the log |

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

## Local Development

### Build from Source

If you want to build the MCP server from source code (for development or contribution purposes), run the following command from the source code root directory:

```bash
make build
```

This creates a `bin` directory containing the binary named `mcp-server`. Copy the absolute path to the binary to use in your MCP server configuration.

## License

The code is licensed under an [Apache 2.0](./LICENSE) license.
