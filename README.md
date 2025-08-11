<div align="center">
<img src="https://raw.githubusercontent.com/perses/perses/main/docs/images/perses_logo_cropped.svg" alt="Perses">
    <h1 align="center">MCP Server for Perses</h1>
</div>

> [!WARNING]  
> This MCP Server is currently in **beta**. Features and tools may change, and stability is not guaranteed. Feedback and contributions are most welcome!

## Overview

The Perses MCP Server is a local [Model Context Protocol (MCP)](https://modelcontextprotocol.io/introduction) Server that enables the LLM hosts(Claude Desktop, VS Code, Cursor) to interact with the Perses Application in a standardized way.

## Demo

<details open>
<summary> Claude Desktop</summary>
  
https://github.com/user-attachments/assets/87137515-1b45-442d-a4c9-68f460a1ba4c
</details>

<details>
<summary>VS Code with GitHub Copilot</summary>

https://github.com/user-attachments/assets/b80c354a-8006-4e1f-b7f4-e123002f7dc3

</details>

## Pre-requisites

- [percli](https://perses.dev/perses/docs/cli/)
- `PERSES_TOKEN`

### Obtaining Your Perses Authentication Token

1. Login to your Perses server using the `percli` command line tool:

```bash
percli login <PERSES_SERVER_URL>
```

For example, `percli login https://demo.perses.dev`.

Or `percli login http://localhost:8080` if you are running [perses/perses](https://github.com/perses/perses) locally from the source code or from the perses image.

2. After successful login, retrieve your token:

```bash
percli whoami --show-token
```

Copy the token to use in your MCP server configuration.

**WARNING: Your login will automatically expire in 15 minutes**. If you want to extend the token duration, you can change the `access_token_ttl` setting in the Perses app [configuration](https://perses.dev/perses/docs/configuration/configuration/?h=configu), then restart the app (if running locally) or rebuild the Docker image.

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

In this mode, the MCP server communicates with the LLM host via standard input and output (STDIO). For more details, see the [MCP Protocol Specification docs](https://modelcontextprotocol.io/specification/2025-06-18/basic/transports#stdio).

<details>
<summary>Install in Claude Desktop</summary>

https://github.com/user-attachments/assets/b80c354a-8006-4e1f-b7f4-e123002f7dc3

To add this MCP server to [Claude Desktop](https://claude.ai/download):

1. Create or edit the Claude Desktop configuration file at:

   - macOS: `~/Library/Application Support/Claude/claude_desktop_config.json`
   - Windows: `%APPDATA%\Claude\claude_desktop_config.json`
   - Linux: `~/.config/Claude/claude_desktop_config.json`

You can easily access this file via the Claude Desktop app by navigating to `Claude > Settings > Developer > Edit Config`.

2. Add the following JSON block to the configuration file:

```json
{
  "mcpServers": {
    "perses-mcp": {
      "command": "<ABSOLUTE_PATH_TO_PERSES_MCP_BINARY>",
      "args": [
        "--perses-server-url",
        "<PERSES_SERVER_URL>"
        // Add "--read-only" here for read-only mode
      ],
      "env": {
        "PERSES_TOKEN": "<PERSES_TOKEN>"
      }
    }
  }
}
```

3. Restart Claude Desktop for the changes to take effect.
</details>

<details>
<summary>Install in VS Code</summary>


Add the following JSON code snippet to the VS Code MCP Config file. See [VS Code MCP documentation](https://code.visualstudio.com/docs/copilot/chat/mcp-servers) for more details.

```json
{
  "inputs": [
    {
      "type": "promptString",
      "id": "perses-token",
      "description": "PERSES_TOKEN to connect with Perses Application",
      "password": true
    }
  ],
  "servers": {
    "perses-mcp": {
      "command": "<ABSOLUTE_PATH_TO_PERSES_MCP_BINARY>",
      "args": [
        "--perses-server-url",
        "http://localhost:8080"
        // Add "--read-only" here for read-only mode
      ],
      "env": {
        "PERSES_TOKEN": "${input:perses-token}"
      }
    }
  }
}
```
</details>


### Streamable HTTP Mode
The Streamable HTTP mode allows the MCP server to communicate with LLM hosts over HTTP, similar to a regular web API. This mode is particularly useful for:

- **Remote hosting**: Deploy the MCP server on a cloud instance or remote server
- **Multiple clients**: Allow multiple LLM hosts to connect to the same server instance

For technical details, see the [MCP Protocol Specification](https://modelcontextprotocol.io/specification/2025-06-18/basic/transports#streamable-http).


Before starting the MCP server, set your Perses authentication token:

```bash
export PERSES_TOKEN=<YOUR_PERSES_TOKEN>
```

Run the following command to start the MCP server in Streamable HTTP mode:

```bash
/path/to/mcp-server --transport streamable-http --perses-server-url <PERSES_SERVER_URL> --port 8000
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

## Command-Line Flags

The Perses MCP Server supports several command-line flags to customize its behavior:

| Flag | Default | Description |
|------|---------|-------------|
| `--perses-server-url` | `http://localhost:8080` | The Perses backend server URL |
| `--log-level` | `info` | Log level (options: `debug`, `info`, `warn`, `error`) |
| `--transport` | `stdio` | MCP protocol transport mechanism (options: `stdio`, `streamable-http`) |
| `--port` | `8000` | Port to run the HTTP Streamable server on (only used with `streamable-http` transport) |
| `--read-only` | `false` | Restrict the server to read-only operations |

## Local Development

### Build from Source

If you want to build the MCP server from source code (for development or contribution purposes), run the following command from the source code root directory:

```bash
make build
```

This should create a `bin` directory which contains the binary named `mcp-server`. Copy the absolute path to the binary to use in your MCP server configuration.

## Tools

> [!NOTE]  
> When running in read-only mode (`--read-only` flag), only tools that retrieve information are available. Write operations like `create_project`, `create_dashboard`, `create_global_datasource`, `update_global_datasource`, and `create_project_variable` are disabled in read-only mode.

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
