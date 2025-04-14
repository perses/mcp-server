<div align="center">
<img src="https://raw.githubusercontent.com/perses/perses/main/docs/images/perses_logo_cropped.svg" alt="Perses">
    <h1 align="center">MCP Server for Perses</h1>
</div>

## Overview

The Perses MCP Server is a local [Model Context Protocol (MCP)](https://modelcontextprotocol.io/introduction) Server that enables the LLM hosts(Claude Desktop, VS Code, Cursor) to interact with the Perses Application in a standardized way.

## Demo

<details>
<summary> Claude Desktop</summary>
  
https://github.com/user-attachments/assets/d0ba1b03-42a1-4d70-9bb3-5a56c4193e93
</details>

<details>
<summary>VS Code with GitHub Copilot</summary>

https://github.com/user-attachments/assets/b80c354a-8006-4e1f-b7f4-e123002f7dc3
</details>


## Usage

### Integration with Claude Desktop

To add this MCP server to Claude Desktop:

1. Create or edit the Claude Desktop configuration file at:

   - macOS: `~/Library/Application Support/Claude/claude_desktop_config.json`
   - Windows: `%APPDATA%\Claude\claude_desktop_config.json`
   - Linux: `~/.config/Claude/claude_desktop_config.json`
   
You can easily access this file via the Claude Desktop app by navigating to `Claude > Settings > Developer > Edit Config`.

2. Add the following lines to the configuration file:

```json
{
  "mcpServers": {
    "perses-mcp": {
      "command": "/Users/I513945/development/contribution/perses-workspace/perses-mcp/bin/perses-mcp",
      "args": [
        "--perses-server-url",
        "<PERSES_SERVER_URL>"
      ],
      "env": {
        "PERSES_TOKEN": "<PERSES_TOKEN>"
      }
    },
  }
}

```
3. Restart Claude Desktop for the changes to take effect.

### Integration with Vs Code GitHub Copilot
tbd

## Tools

### Projects

| Tool                   | Description       | Required Parameters |
| ---------------------- | ----------------- | ------------------- |
| `perses_list_projects` | List all projects | -                   |

### Dashboards

| Tool                     | Description                                | Required Parameters |
| ------------------------ | ------------------------------------------ | ------------------- |
| `perses_list_dashboards` | List all dashboards for a specific project | `project`           |


### Datasources

| Tool                             | Description                                 | Required Parameters |
| -------------------------------- | ------------------------------------------- | ------------------- |
| `perses_list_global_datasources` | List all global datasources                 | -                   |
| `perses_list_datasources`        | List all datasources for a specific project | `project`           |

### Variables

| Tool                           | Description                               | Required Parameters |
| ------------------------------ | ----------------------------------------- | ------------------- |
| `perses_list_global_variables` | List all global variables                 | -                   |
| `perses_list_variables`        | List all variables for a specific project | `project`           |


