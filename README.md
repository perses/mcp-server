<div align="center">
<img src="https://raw.githubusercontent.com/perses/perses/main/docs/images/perses_logo_cropped.svg" alt="Perses">
    <h1 align="center">MCP Server for Perses</h1>
</div>

> [!WARNING]  
> This MCP Server is currently in **beta**. Features and tools may change, and stability is not guaranteed. Feedback and contributions are most welcome!


## Overview

The Perses MCP Server is a local [Model Context Protocol (MCP)](https://modelcontextprotocol.io/introduction) Server that enables the LLM hosts(Claude Desktop, VS Code, Cursor) to interact with the Perses Application in a standardized way.

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="https://github.com/user-attachments/assets/d691a924-c53d-408d-a3f9-a029f3fa6dc2">
  <source media="(prefers-color-scheme: light)" srcset="https://github.com/user-attachments/assets/416409df-9045-41f3-b10b-91df3020af1f">
  <img alt="Fallback image description" src="https://github.com/user-attachments/assets/416409df-9045-41f3-b10b-91df3020af1f">
</picture>

## Demo

<details open>
<summary> Claude Desktop</summary>
  
https://github.com/user-attachments/assets/87137515-1b45-442d-a4c9-68f460a1ba4c
</details>

<details>
<summary>VS Code with GitHub Copilot</summary>

https://github.com/user-attachments/assets/b80c354a-8006-4e1f-b7f4-e123002f7dc3
</details>


## Usage

### Pre-requisites
- [percli](https://perses.dev/perses/docs/cli/)
- `PERSES_TOKEN`

#### Obtaining Your Perses Authentication Token

1. Login to your Perses server using the `percli` command line tool:

```bash
percli login <PERSES_SERVER_URL>
```

2. After successful login, retrieve your token:
```bash
percli whoami --show-token
```

1. Copy the token to use in your MCP server configuration.

### Integration with Claude Desktop

To add this MCP server to Claude Desktop:

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
      ],
      "env": {
        "PERSES_TOKEN": "<PERSES_TOKEN>"
      }
    }
  }
}
```
3. Restart Claude Desktop for the changes to take effect.

### Integration with VS Code GitHub Copilot

To integrate the MCP server with VS Code GitHub Copilot, follow these steps:

1. Open User Settings (JSON) in VS Code:
   - Press `Cmd + Shift + P` (on macOS) or `Ctrl + Shift + P` (on other platforms).
   - Type `Preferences: Open User Settings (JSON)` and select it.

2. Add the following JSON block to the User Settings (JSON) file:

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
      ],
      "env": {
        "PERSES_TOKEN": "${input:perses-token}"
      }
    }
  }
}
```

1. Optionally, create a file named `.vscode/mcp.json` in your workspace and add the same JSON block. This allows you to share the configuration with others.

## Tools

### Projects

| Tool                         | Description           | Required Parameters |
| ---------------------------- | --------------------- | ------------------- |
| `perses_list_projects`       | List all projects     | -                   |
| `perses_get_project_by_name` | Get a project by name | `project`           |

### Dashboards

| Tool                           | Description                                | Required Parameters    |
| ------------------------------ | ------------------------------------------ | ---------------------- |
| `perses_list_dashboards`       | List all dashboards for a specific project | `project`              |
| `perses_get_dashboard_by_name` | Get a dashboard by name for a project      | `project`, `dashboard` |

### Datasources

| Tool                                    | Description                                 | Required Parameters     |
| --------------------------------------- | ------------------------------------------- | ----------------------- |
| `perses_list_global_datasources`        | List all global datasources                 | -                       |
| `perses_list_datasources`               | List all datasources for a specific project | `project`               |
| `perses_get_global_datasource_by_name`  | Get a global datasource by name             | `datasource`            |
| `perses_get_project_datasource_by_name` | Get a project datasource by name            | `project`, `datasource` |

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

## License

The code is licensed under an [Apache 2.0](./LICENSE) license.
