# MCP clients

This page explains how to connect the Perses MCP Server to the most common LLM hosts. There are two ways of doing it,
depending on the [transport](./configuration.md#definition) used by the MCP server:

- [stdio](#stdio): the LLM host starts the MCP server itself, on your machine.
- [Streamable HTTP](#streamable-http): the MCP server is already running, and the LLM host connects to its URL.

See [Deployment](./deployment.md) to choose between them.

## stdio

In this mode, the LLM host needs the absolute path to the `perses-mcp-server` binary (see [Installation](./installation.md))
and, optionally, the absolute path to a [configuration file](./configuration.md#configuration-file).

Most of the LLM hosts (Claude Desktop, Cursor...) accept the following configuration:

```json
{
  "mcpServers": {
    "perses": {
      "command": "<ABSOLUTE_PATH_TO_PERSES_MCP_SERVER>",
      "args": [
        "--config",
        "<ABSOLUTE_PATH_TO_CONFIG_YAML>"
      ],
      "env": {
        "PERMCP_PERSES_SERVER_NATIVE_AUTH_LOGIN": "<LOGIN>",
        "PERMCP_PERSES_SERVER_NATIVE_AUTH_PASSWORD": "<PASSWORD>"
      }
    }
  }
}
```

> [!TIP]
> Pass the sensitive values (passwords, tokens) as [environment variables](./configuration.md#environment-variables)
> instead of writing them in the configuration file. The configuration file is optional: the MCP server can be
> configured with environment variables only.

### OpenCode

Add the following to your [OpenCode configuration](https://opencode.ai/docs/config/). See the
[OpenCode documentation](https://opencode.ai/docs/mcp-servers/) for more details.

```json
{
  "$schema": "https://opencode.ai/config.json",
  "mcp": {
    "perses": {
      "type": "local",
      "command": [
        "<ABSOLUTE_PATH_TO_PERSES_MCP_SERVER>",
        "--config",
        "<ABSOLUTE_PATH_TO_CONFIG_YAML>"
      ],
      "environment": {
        "PERMCP_PERSES_SERVER_NATIVE_AUTH_LOGIN": "{env:PERMCP_PERSES_SERVER_NATIVE_AUTH_LOGIN}",
        "PERMCP_PERSES_SERVER_NATIVE_AUTH_PASSWORD": "{env:PERMCP_PERSES_SERVER_NATIVE_AUTH_PASSWORD}"
      },
      "enabled": true
    }
  }
}
```

### Claude Code

Use the Claude Code CLI to add the MCP server. See the
[Claude Code documentation](https://code.claude.com/docs/en/mcp) for more details.

```bash
claude mcp add --transport stdio \
  --env PERMCP_PERSES_SERVER_NATIVE_AUTH_LOGIN=<LOGIN> \
  --env PERMCP_PERSES_SERVER_NATIVE_AUTH_PASSWORD=<PASSWORD> \
  perses -- <ABSOLUTE_PATH_TO_PERSES_MCP_SERVER> --config <ABSOLUTE_PATH_TO_CONFIG_YAML>
```

### Claude Desktop

Create or edit the Claude Desktop configuration file. You can open it from `Claude > Settings > Developer > Edit Config`,
or find it at:

- macOS: `~/Library/Application Support/Claude/claude_desktop_config.json`
- Windows: `%APPDATA%\Claude\claude_desktop_config.json`
- Linux: `~/.config/Claude/claude_desktop_config.json`

Add the [standard configuration](#stdio) above to this file, then restart Claude Desktop.

### VS Code

Add the following to your VS Code MCP configuration file (`.vscode/mcp.json` in a workspace, or with the command
`MCP: Open User Configuration`). See the
[VS Code documentation](https://code.visualstudio.com/docs/agent-customization/mcp-servers) for more details.

```json
{
  "servers": {
    "perses": {
      "type": "stdio",
      "command": "<ABSOLUTE_PATH_TO_PERSES_MCP_SERVER>",
      "args": [
        "--config",
        "<ABSOLUTE_PATH_TO_CONFIG_YAML>"
      ],
      "env": {
        "PERMCP_PERSES_SERVER_NATIVE_AUTH_LOGIN": "<LOGIN>",
        "PERMCP_PERSES_SERVER_NATIVE_AUTH_PASSWORD": "<PASSWORD>"
      }
    }
  }
}
```

### Cursor

Add the [standard configuration](#stdio) above to `~/.cursor/mcp.json` (for all your projects) or to `.cursor/mcp.json`
(for a single project). See the [Cursor documentation](https://cursor.com/docs/mcp) for more details.

### Using the container image

Instead of a binary, the LLM host can start the [container image](./installation.md#container-images). The option `-i`
keeps the standard input open, and each `-e` option without a value forwards the corresponding environment variable to
the container:

```json
{
  "mcpServers": {
    "perses": {
      "command": "docker",
      "args": [
        "run", "-i", "--rm",
        "-e", "PERMCP_PERSES_SERVER_URL",
        "-e", "PERMCP_PERSES_SERVER_NATIVE_AUTH_LOGIN",
        "-e", "PERMCP_PERSES_SERVER_NATIVE_AUTH_PASSWORD",
        "persesdev/mcp-server"
      ],
      "env": {
        "PERMCP_PERSES_SERVER_URL": "https://perses.example.com",
        "PERMCP_PERSES_SERVER_NATIVE_AUTH_LOGIN": "<LOGIN>",
        "PERMCP_PERSES_SERVER_NATIVE_AUTH_PASSWORD": "<PASSWORD>"
      }
    }
  }
}
```

## Streamable HTTP

In this mode, the MCP server is started beforehand with `transport: http` (see [Deployment](./deployment.md)), and the
LLM host only needs its URL. The MCP server answers on any path: this documentation uses `/mcp` by convention, for
example `http://localhost:8000/mcp`.

If the MCP server is behind a reverse proxy requiring an authentication, most of the LLM hosts let you add HTTP headers
to the requests, as shown below.

### OpenCode

```json
{
  "$schema": "https://opencode.ai/config.json",
  "mcp": {
    "perses": {
      "type": "remote",
      "url": "http://localhost:8000/mcp",
      "oauth": false,
      "enabled": true
    }
  }
}
```

`oauth: false` disables the automatic OAuth discovery of OpenCode, as the MCP server doesn't implement it. Use the field
`headers` to send additional HTTP headers.

### Claude Code

```bash
claude mcp add --transport http perses http://localhost:8000/mcp
```

Use the option `--header` to send additional HTTP headers.

### VS Code

```json
{
  "servers": {
    "perses": {
      "type": "http",
      "url": "http://localhost:8000/mcp"
    }
  }
}
```

Use the field `headers` to send additional HTTP headers.

### Cursor

```json
{
  "mcpServers": {
    "perses": {
      "url": "http://localhost:8000/mcp"
    }
  }
}
```

Use the field `headers` to send additional HTTP headers.
