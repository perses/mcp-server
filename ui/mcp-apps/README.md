# MCP Apps views

The [MCP Apps](https://modelcontextprotocol.io/docs/extensions/apps) views of the Perses MCP server, rendered inline by
the clients that support MCP Apps.

- `app/panels.html`: the view of `perses_render_panels` (`ui://perses/panels.html`). It renders the panels of the tool
  result with the Perses panel plugins, and loads their data with the app-only tool `perses_query_prometheus_data`
  (see `app/prometheusFetch.ts`).

The views are built into single HTML files in `dist/` and embedded in the server binary (`ui/ui.go`). `dist/` is not
committed, except a placeholder that keeps the embed pattern valid:

- `make build` builds the views before the server when `npm` is installed (Node.js: see `.nvmrc`). Without it, the
  server is built without the views: it works, but doesn't have `perses_render_panels`, and says so when it starts.
- The releases (`make cross-build`, `make cross-release`) always build the views, and fail when the server doesn't
  embed them.

```bash
make build-mcp-apps   # npm ci && npm run build
make test-mcp-apps    # typecheck and Vitest
make checkmcpapps     # build the views and check that the server embeds them
```

The CI runs the last two. The panel plugins bundled in `app/plugins.ts` must be the panel
kinds accepted by the server (`panelKinds` in `pkg/tools/prometheus/panelkinds.go`): a test checks it.

## Updating the Perses plugins

The panels are the npm packages of [perses/plugins](https://github.com/perses/plugins), at the versions of
`package.json`: a release of a plugin changes nothing here until its version is updated and the views are rebuilt. The
view therefore shows the plugins of the server release, which may not be the versions installed in Perses.

Dependabot opens a weekly pull request that updates the `@perses-dev/*` packages together. The CI builds the views
with the new versions and tests them, and the next release embeds them: nothing else is needed. By hand: update the
`@perses-dev/*` versions together (they depend on each other, see their `peerDependencies`), run `npm install`, then
`make test-mcp-apps`, and commit `package.json` and `package-lock.json`. A new minor version of Perses may change the
API used in `app/`: check the view in a host (see below) before merging.

## Testing locally

The unit tests (`make test-mcp-apps`) don't render the view in a host. To see it, follow the
[local development](https://apps.extensions.modelcontextprotocol.io/api/#local-development) guide of MCP Apps: let an
MCP client that supports MCP Apps rebuild and start the server over stdio. All the setups below need a Perses with a
Prometheus datasource (see [Configuration](../../docs/configuration.md)).

A client that doesn't support MCP Apps (e.g. Claude Code) only shows the text content of `perses_render_panels`, which
is also worth checking.

### Claude Desktop, VS Code and other MCP clients

The client runs a command that rebuilds the view and the server, then starts the server over stdio, so that each new
session runs the current sources. Claude Desktop (`claude_desktop_config.json`, see [Clients](../../docs/clients.md)):

```json
{
  "mcpServers": {
    "perses-dev": {
      "command": "bash",
      "args": ["-c", "cd /path/to/mcp-server && make build >&2 && exec ./bin/perses-mcp-server --config perses-mcp-config.yaml"]
    }
  }
}
```

- The configuration must use `transport: stdio`; `read_only: true` is enough.
- stdout carries the MCP messages: send the output of the build to stderr (`>&2`), as above. Claude Desktop writes
  stderr to `~/Library/Logs/Claude/mcp-server-perses-dev.log` (macOS).
- The clients start the command with a minimal `PATH`: add the folders of `go`, `node` and `npm` to it if they are not
  found (e.g. `export PATH=/opt/homebrew/bin:$PATH && cd ...`).
- `make build` rebuilds the views only when their sources changed.

Restart the client after a change, then ask a question that needs a chart, e.g. "show me the scrape duration of my
Prometheus targets over the last hour". For VS Code, add the same command to `.vscode/mcp.json` (`"type": "stdio"`).

Check that the panels are displayed, that changing the time range or refreshing reloads them (through
`perses_query_prometheus_data`), that the view follows the theme of the client, and the fullscreen mode.

### basic-host of ext-apps

[basic-host](https://github.com/modelcontextprotocol/ext-apps/tree/main/examples/basic-host), the reference host of
the MCP Apps SDK, helps to debug the view: you call a tool with a JSON input, and it shows the view, the tool input and
result, and logs the messages between the view and the host in the browser console (`[HOST]`). Start it as described
in its README, with the URL of the server (HTTP transport) in `SERVERS`.

basic-host connects to the MCP servers from the browser, and the server doesn't send CORS headers: put a CORS proxy in
front of it. Its ports may also be taken by Perses: it listens on 8080 (`HOST_PORT`) and loads its sandbox from
`http://localhost:8081`.

An input for `perses_render_panels` (add `"datasource": {"project": "<project>"}` when Perses has no default global
Prometheus datasource):

```json
{"panels": [
  {"title": "Scrape duration", "unit": "seconds", "queries": [{"promql": "scrape_duration_seconds", "legend": "{{job}}"}]},
  {"title": "Targets up", "kind": "StatChart", "queries": [{"promql": "sum(up)"}]},
  {"title": "Samples by job", "kind": "BarChart", "queries": [{"promql": "sum by (job) (scrape_samples_scraped)", "legend": "{{job}}"}]}
]}
```

### Remote hosts (claude.ai, ChatGPT)

Hosts that run in the cloud can't reach a local server: expose the HTTP transport with a tunnel, e.g.
`npx cloudflared tunnel --url http://127.0.0.1:8000`, and add `https://<name>.trycloudflare.com/mcp` as a remote MCP
server. The URL changes when the tunnel restarts. The server has no authentication of its own: anybody who has the URL
can use it with the credentials of its configuration, so keep `read_only: true` and stop the tunnel after the test.
