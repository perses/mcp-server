# Perses MCP Server
The Perses MCP Server is a local [Model Context Protocol (MCP)](https://modelcontextprotocol.io/introduction) Server for the Perses Application. This server enables the LLM hosts(Claude Desktop, VS Code, Cursor) that support MCP to interact with the Perses Application in a standardized way.

## Tools


### Projects

| Tool                   | Description                                 | Required Parameters |
| ---------------------- | ------------------------------------------- | ------------------- |
| `perses_list_projects` | List all `Projects` in a Perses Application | -                   |

### Dashboards

| Tool                     | Description                                  | Required Parameters |
| ------------------------ | -------------------------------------------- | ------------------- |
| `perses_list_dashboards` | List all `Dashboards` for a specific project | `project`           |


### Datasources

| Tool                             | Description                                   | Required Parameters |
| -------------------------------- | --------------------------------------------- | ------------------- |
| `perses_list_global_datasources` | List all `Global Datasources`                 | -                   |
| `perses_list_datasources`        | List all `Datasources` for a specific project | `project`           |

### Variables

| Tool                           | Description                                 | Required Parameters |
| ------------------------------ | ------------------------------------------- | ------------------- |
| `perses_list_global_variables` | List all `Global Variables`                 | -                   |
| `perses_list_variables`        | List all `Variables` for a specific project | `project`           |


