# Perses MCP Server
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


