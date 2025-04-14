# Perses MCP Server
The Perses MCP Server is a local [Model Context Protocol (MCP)](https://modelcontextprotocol.io/introduction) Server that enables the LLM hosts(Claude Desktop, VS Code, Cursor) to interact with the Perses Application in a standardized way.

## Demo

### Claude desktop
https://github.com/user-attachments/assets/d0ba1b03-42a1-4d70-9bb3-5a56c4193e93

### VS Code with GitHub Copilot






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


