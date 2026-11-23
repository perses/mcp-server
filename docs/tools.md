# Tools

This page lists the tools provided by the Perses MCP Server. The tools are grouped by
[resource](./configuration.md#resources): use the setting `resources` to only register some of them.

> [!NOTE]
> In [read-only mode](./configuration.md#definition) (`read_only: true`), the tools creating, updating or deleting
> resources (`perses_create_*`, `perses_update_*`, `perses_delete_*` and `perses_set_*`) are not registered. Read and
> validation tools stay available.

Whatever the tools registered, they can only do what the identity configured in the MCP server is allowed to do in
Perses (see [Permissions](./overview.md#permissions)).

## Projects

Resource: `project`

| Tool                         | Description                | Required parameters | Optional parameters          |
|------------------------------|----------------------------|---------------------|------------------------------|
| `perses_list_projects`       | List all projects          | -                   | -                            |
| `perses_get_project_by_name` | Get a project by name      | `name`              | -                            |
| `perses_create_project`      | Create a new project       | `project`           | `displayName`, `description` |
| `perses_update_project`      | Update an existing project | `name`              | `displayName`, `description` |
| `perses_delete_project`      | Delete a project           | `name`              | -                            |

## Dashboards

Resource: `dashboard`

| Tool                           | Description                                | Required parameters    |
|--------------------------------|--------------------------------------------|------------------------|
| `perses_list_dashboards`       | List all dashboards of a project           | `project`              |
| `perses_get_dashboard_by_name` | Get an outline by default; use `summary: false` for complete JSON before read-modify-write, or `panels: [keys]` for those full panels regardless of `summary`; unknown keys are listed in `missing` with the `available` keys | `project`, `name` |
| `perses_create_dashboard`      | Create a dashboard in a project            | `project`, `dashboard` |
| `perses_update_dashboard`      | Update an existing dashboard of a project  | `project`, `dashboard` |
| `perses_delete_dashboard`      | Delete a dashboard from a project          | `project`, `name`      |
| `perses_set_dashboard_panels` | Add, replace or `remove` panels (with their layout items); new panels are appended to a panel `group` | `project`, `name`; at least one entry in `panels` or `remove` |
| `perses_set_dashboard_variables` | Add, replace or `remove` variables by name | `project`, `name`; at least one entry in `variables` or `remove` |
| `perses_set_dashboard_layout` | Define, replace or `remove` a panel group | `project`, `name`, `title` |

The parameter `dashboard` is a dashboard in the [Perses format](https://perses.dev/docs/perses/latest/api/dashboard/).
Create and update take the dashboard as a JSON object; the Perses server validates it when saving it, and a
validation failure is a tool error with the error of the Perses client, unchanged. Create fails if a
dashboard with the same name exists; use `perses_update_dashboard` to change it. Writes return a compact summary; set
`include_dashboard: true` to include the saved dashboard, or with `dry_run: true` (validate without saving) the
dashboard that would be saved.

The incremental tools fetch the current dashboard, apply the requested changes and save it; Perses validates it when
saving, and `dry_run: true` validates it without saving.
In `perses_set_dashboard_panels` and `perses_set_dashboard_variables`, `remove` is applied before additions in the
same write and must not name a key that is also being set. Invalid edits do not persist any changes.
The edits made through one MCP server, including `perses_update_dashboard`, are serialized, so parallel tool calls
do not lose each other's changes. `expected_version` checks the version before editing to detect intervening changes
by other writers; it is not an atomic server-side lock, so a concurrent write after the check can still race.
Decoding errors name the element they come from (`panels."<key>": ` or `variables[<index>]: `), followed by the
message of the Perses model.

Grid layouts are edited as panel groups, like the Perses
[Go SDK](https://github.com/perses/perses/blob/main/go-sdk/dashboard/options.go) (`AddPanelGroup`) and
[CUE DaC utils](https://github.com/perses/perses/tree/main/cue/dac-utils/panelgroup) build them. Every panel of a
group has the same size, `width = 24 / panels_per_line` (rounded down) and `panel_height` rows of 30px (at most 48,
already taller than most screens), and the panels are laid out in order: panel `i` is at `x = width * (i % panels_per_line)`, `y = panel_height * (i / panels_per_line)`.
For example, 4 panels with `panels_per_line: 3` and `panel_height: 6` are placed at `(0,0)`, `(8,0)`, `(16,0)` and `(0,6)`.

- `perses_set_dashboard_layout` defines or replaces the Grid group with the given `title` (an exact match, else
  ignoring case; several groups matching the same way are an error) from an ordered list of existing `panels`. A
  replaced group keeps its title. `panels_per_line`, `panel_height` and `collapsed` default to those of the replaced
  group, else 2 panels per line, height 8, not collapsible. Listed panels move out of other Grid groups, and those
  laid out with the formula above are laid out again. Panels left out are reported as no longer displayed, and listed
  panels that a tab also displays are reported too.
  `remove: true` deletes the group and keeps its panels, reported as no longer displayed; it takes no other option.
- `perses_set_dashboard_panels` keeps the layout item of a replaced panel. New panels are appended, in key order, to
  the Grid group titled `group` (the last group by default, created if missing; without `group` and without any
  Grid group, a group titled `Panel Group` is created), with the size of its first panel.
  When the items of the group are where the formula above puts them, the group is laid out again with its panels per
  line (`24 / width`) and height. Otherwise the group was arranged by hand: its items keep their positions and the new
  panels fill rows below its lowest item. Removed panels lose their layout items, and the groups that were laid out
  with the formula are laid out again to close the gaps; groups arranged by hand keep their positions.

Tabs layouts are left untouched. Custom positions need `perses_update_dashboard`.

`perses_update_dashboard` reads the dashboard first only for `expected_version` or `dry_run`, so a plain
update needs only the update permission.

`perses_validate_dashboard` checks the JSON against installed plugin CUE schemas without saving it.
`perses_get_plugin_schema` provides plugin field definitions.

## Ephemeral dashboards

Resource: `ephemeraldashboard`

[Ephemeral dashboards](https://perses.dev/docs/perses/latest/concepts/ephemeral-dashboard/) are temporary dashboards
with a time-to-live (TTL): Perses deletes them automatically once expired. They are handy to let the LLM build a
dashboard to answer a question, without cluttering the project.

| Tool                                     | Description                                         | Required parameters    |
|------------------------------------------|-----------------------------------------------------|------------------------|
| `perses_list_ephemeral_dashboards`       | List all ephemeral dashboards of a project          | `project`              |
| `perses_get_ephemeral_dashboard_by_name` | Get an ephemeral dashboard by name                  | `project`, `name`      |
| `perses_create_ephemeral_dashboard`      | Create an ephemeral dashboard in a project          | `project`, `dashboard` |
| `perses_update_ephemeral_dashboard`      | Update an existing ephemeral dashboard of a project | `project`, `dashboard` |
| `perses_delete_ephemeral_dashboard`      | Delete an ephemeral dashboard from a project        | `project`, `name`      |

The parameter `dashboard` is an ephemeral dashboard in the
[Perses format](https://perses.dev/docs/perses/latest/api/ephemeral-dashboard/).

To check an ephemeral dashboard without saving it, pass `perses_validate_dashboard` a copy with kind `Dashboard` and
`spec.ttl` omitted (TTL is not validated).
`perses_get_plugin_schema` provides plugin field definitions.

## Datasources

Resources: `globaldatasource` for the tools `*_global_datasource*`, `datasource` for the tools `*_project_datasource*`.

| Tool                                    | Description                                                | Required parameters          | Optional parameters                               |
|-----------------------------------------|------------------------------------------------------------|------------------------------|---------------------------------------------------|
| `perses_list_global_datasources`        | List all global datasources                                | -                            | -                                                 |
| `perses_get_global_datasource_by_name`  | Get a global datasource by name                            | `name`                       | -                                                 |
| `perses_query_global_datasource`        | Send an HTTP request to a global datasource through Perses | `datasource_name`            | `method`, `path`, `query_params`, `body`, `headers` |
| `perses_create_global_datasource`       | Create a new global datasource                             | `name`, `type`, `url`        | `display_name`, `proxy_type`                      |
| `perses_update_global_datasource`       | Update an existing global datasource                       | `name`, `type`, `url`        | `display_name`, `proxy_type`                      |
| `perses_delete_global_datasource`       | Delete a global datasource                                 | `name`                       | -                                                 |
| `perses_list_project_datasources`       | List all datasources of a project                          | `project`                    | -                                                 |
| `perses_get_project_datasource_by_name` | Get a project datasource by name                           | `project`, `name`            | -                                                 |
| `perses_query_project_datasource`       | Send an HTTP request to a project datasource through Perses | `project`, `datasource_name` | `method`, `path`, `query_params`, `body`, `headers` |
| `perses_create_project_datasource`      | Create a new datasource in a project                       | `project`, `datasource`      | -                                                 |
| `perses_update_project_datasource`      | Update an existing datasource of a project                 | `project`, `datasource`      | -                                                 |
| `perses_delete_project_datasource`      | Delete a datasource from a project                         | `project`, `name`            | -                                                 |

The parameter `datasource` is a datasource in the [Perses format](https://perses.dev/docs/perses/latest/api/datasource/).

### Querying datasources

The tools `perses_query_global_datasource` and `perses_query_project_datasource` forward an HTTP request to the backend
of a datasource (Prometheus, Loki, Tempo...) through the [Perses proxy](https://perses.dev/docs/perses/latest/concepts/proxy/).
This means:

- **The credentials are injected by Perses.** The secrets configured on the datasource in Perses are applied on the
  server side: they are never exposed to the LLM.
- **The allowed endpoints are enforced.** Only the endpoints allowed by the `allowedEndpoints` of the datasource can be
  reached.

The parameters are:

- `method`: the HTTP method, `GET` (default) or `POST`.
- `path`: the path of the endpoint, relative to the datasource. For example `/api/v1/query_range` for Prometheus, or
  `/loki/api/v1/query_range` for Loki.
- `query_params`: the query string parameters, as an object of key/value strings.
- `body`: the raw body of the request (JSON or `application/x-www-form-urlencoded`). The `Content-Type` is
  `application/json` by default when a body is provided.
- `headers`: additional HTTP headers. The header `Authorization` is always ignored: the authentication to the datasource
  is configured in Perses.

On success (HTTP 2xx), the tool returns the response body of the datasource unchanged, like the other tools return the
answer of Perses: for example the JSON document `{"status":"success","data":{...}}` of the Prometheus API. The response
headers are not returned. On an HTTP error status, the tool returns an error (`isError: true`) that contains the HTTP
status and the response body.

The response body is truncated to 1 MiB, to avoid exceeding the context of the LLM. When the body is truncated, the
marker `[response truncated: exceeded 1 MiB limit]` is appended on a new line, so that the truncation is visible.

Example of an instant query on a global Prometheus datasource:

```json
{
  "datasource_name": "my-prometheus",
  "method": "POST",
  "path": "/api/v1/query",
  "query_params": {
    "query": "up",
    "time": "2024-01-01T00:00:00Z"
  }
}
```

Example of a range query on a Prometheus datasource of a project:

```json
{
  "project": "my-project",
  "datasource_name": "my-prometheus",
  "method": "POST",
  "path": "/api/v1/query_range",
  "query_params": {
    "query": "rate(http_requests_total[5m])",
    "start": "2024-01-01T00:00:00Z",
    "end": "2024-01-01T01:00:00Z",
    "step": "60"
  }
}
```

## Prometheus

Resource: `prometheus`

These read-only tools discover and query the metrics of the Prometheus datasources of Perses. They call the
[Prometheus HTTP API](https://prometheus.io/docs/prometheus/latest/querying/api/) through the Perses proxy, like
`perses_query_*_datasource` (credentials injected by Perses, `allowedEndpoints` enforced), but return compact JSON
instead of the raw response: a range query over 6 hours returning 61 series is about 3 KB with
`perses_query_prometheus` (top 20 series, statistics) and 240 KB with `perses_query_global_datasource` (raw points).

| Tool                                    | Description                                              | Required parameters | Optional parameters                                                                       |
|-----------------------------------------|----------------------------------------------------------|---------------------|-------------------------------------------------------------------------------------------|
| `perses_list_prometheus_metric_names`   | List metric names                                        | -                   | `datasource`, `match`, `search`, `time_range`, `limit`, `include_metadata`                |
| `perses_list_prometheus_label_names`    | List label names                                         | -                   | `datasource`, `match`, `time_range`, `limit`                                              |
| `perses_list_prometheus_label_values`   | List the values of a label                               | `label`             | `datasource`, `match`, `search`, `time_range`, `limit`                                    |
| `perses_get_prometheus_metric_metadata` | Get the type, help and unit of 1 to 20 metrics           | `metrics`           | `datasource`                                                                              |
| `perses_query_prometheus`               | Run a PromQL query and get a compact result              | `query`             | `datasource`, `type`, `time`, `time_range`, `step`, `limit`, `time_zone`                  |

The parameters are:

- `datasource`: `{name}` for a global datasource, `{name, project}` for a project datasource. It is optional: see
  [Default datasource](#default-datasource).
- `match`: a series selector, or a list of series selectors OR-ed together, e.g. `"{job=\"prometheus\"}"`.
- `search`: a case-insensitive substring filter, applied by the MCP server to the list returned by Prometheus. It
  doesn't make Prometheus return less: to narrow a large list, use `match`, e.g. `{__name__=~".*memory.*"}`.
- `time_range`: `{past_duration}` (e.g. `"6h"`, `"7d"`), or `{start, end}` in RFC 3339 (`end` defaults to now).
  The past hour by default. Prometheus applies it approximately (by storage block): the lists may include series
  without samples in it, e.g. series that stopped a few hours ago.
- `limit`: the maximum number of returned items. Defaults: 200 metric names (50 with `include_metadata`, also its max), 200
  label names, 100 label values (max 1000), 20 series (max 100).

The list tools return `{datasource, total, returned, truncated, ...}`. A list over 1 MiB is an error that asks to
narrow it with `match` or `time_range`. With `include_metadata: true`, each metric name comes with its type
and help, shortened to 80 characters. The metadata of all the metrics is fetched in one request; when it is over
1 MiB, the metadata of the returned metrics is looked up one by one instead. When it is unavailable, the metrics are
returned without it, with a warning.

`perses_get_prometheus_metric_metadata` looks up each metric under its own name, then under the name of its family
when the family has series with this suffix, as Prometheus matches them: `_total` of counters, `_bucket`, `_count` and
`_sum` of histograms, `_bucket`, `_count`, `_sum`, `_gcount` and `_gsum` of gauge histograms, `_count` and `_sum` of
summaries, `_info` of info metrics (e.g. `http_requests_total` of the counter `http_requests`). A `_created` series
isn't matched, as it holds a timestamp, and neither is `x_total_total` with the family `x_total`.

`perses_query_prometheus` runs an instant query (`type: "instant"`, default) at `time` (default now), or a range query
(`type: "range"`) over `time_range` with `step` (default about 500 points, at least the scrape interval of the
datasource; see [Datasource settings](#datasource-settings)). It returns:

- for an instant query, the value of each series, highest first: `{resultType: "vector", time, seriesCount, returned,
  commonLabels, series: [{labels, value}]}`;
- for a range query, never the raw points but the statistics of each series, highest max first: `{resultType:
  "matrix", timeRange, step, seriesCount, returned, commonLabels, series: [{labels, min, max, avg, last, minAt, maxAt,
  nonFinite}]}`. `min`, `max` and `avg` leave out the NaN and infinite samples, counted in `nonFinite`; `last` is the
  last sample. A series that only has NaN or infinite samples is not returned, with a warning. `minAt` and `maxAt` are
  in RFC 3339 with the offset of `time_zone` (default UTC).

The labels shared by all returned series are returned once in `commonLabels`. The values keep 4 significant digits,
and all the digits of their integer part, so that timestamps and large counters are not rounded (e.g. `12.35`,
`123457`, `1767355200`); NaN and infinities are returned as strings. Native histograms are not summarized: the tool
returns an error that asks to use `histogram_quantile()`, `histogram_count()` or `histogram_sum()`, without `_bucket`
or `le`, e.g. `histogram_quantile(0.99, sum(rate(x[5m])))`. The `warnings` and `infos` of
Prometheus are passed through. The errors of Prometheus are returned unchanged as tool errors, e.g. `bad_data: invalid
parameter "query": 1:9: parse error: ...`; the other HTTP errors, e.g. of Perses, with their status and message. A
response over 1 MiB is an error that asks to aggregate the query. `time` applies to instant queries only, `time_range`
and `step` to range queries only: the other combinations are errors.

Use `perses_query_*_datasource` for the other endpoints of Prometheus and for the other kinds of datasources.

### Default datasource

The parameter `datasource` of the Prometheus tools is resolved as follows:

- `{name}`: the global datasource `name`; `{name, project}`: the datasource `name` of the project. It must be a
  `PrometheusDatasource`.
- `{project}`: the Prometheus datasource of the project marked as default (`spec.default: true`), else the global one.
- omitted: the global Prometheus datasource marked as default.

When no Prometheus datasource is marked as default but there is exactly one that the Perses proxy can query, it is
used. Otherwise the tool returns an error that lists the available Prometheus datasources. The resolved datasources
are cached for 30 seconds.

The tools query the datasources through the Perses proxy: a datasource with a `directUrl`, which only the browser can
reach, is an error, also when it is the default one.

### Datasource settings

Like the Perses UI, the tools apply two settings of the Prometheus datasource, which the Perses proxy doesn't:

- `queryParams` are added to every request, unless the request sets the same parameter, e.g. `dedup` or
  `partial_response` for Thanos. A value with a dashboard variable (`$`) can't be resolved: it is sent as is, with a
  warning.
- `scrapeInterval` (default `1m`) is the minimum default `step` of range queries. An explicit `step` isn't raised to
  the scrape interval, but like the default one it is raised if needed to stay under the Prometheus limit of 11000
  points per series. Before, the minimum default step was 15s: range statistics are now computed over fewer points
  unless `step` is set.

### Panels

| Tool                           | Description                                         | Required parameters | Optional parameters                                          |
|--------------------------------|-----------------------------------------------------|---------------------|--------------------------------------------------------------|
| `perses_render_panels`         | Show Prometheus queries as Perses panels            | `panels`            | `datasource`, `time_range`, `time_zone`, `allow_no_data`     |
| `perses_query_prometheus_data` | Load the data of the panels view (app-only)         | `query`             | `datasource`, `time`, `start`, `end`, `step`                 |

`perses_render_panels` shows data to the user as Perses panels. Each of the 1 to 6 `panels` has:

- `title`, and an optional `description`;
- `kind`: one of the panel plugins of Perses that display time series: `TimeSeriesChart` (default), `StatChart`,
  `GaugeChart`, `BarChart`, `PieChart`, `Table`, `TimeSeriesTable`, `StatusHistoryChart`, `HeatMapChart` and
  `HistogramChart` (the last two display native histograms only). The plugin must also be installed in Perses: the
  list of the plugins of Perses is cached for 5 minutes; when it can't be read, the kinds are not checked against it,
  with a warning;
- `unit`: a unit of the Perses formats, e.g. `bytes`, `seconds`, `percent` (0-100), `percent-decimal` (0-1) or
  `requests/sec`, set in the format of the panel (not for `Table`, `TimeSeriesTable` and `StatusHistoryChart`);
- `queries`: 1 to 5 `{promql, legend}`, where `legend` is a series name with label placeholders, e.g.
  `"{{instance}}"`;
- `spec`: an optional plugin spec, merged over the defaults of the kind, e.g. `{"thresholds": {...}}`.

The panels are validated by Perses (`/api/validate/dashboards`) against the schemas of the plugins: an invalid spec is a
tool error with the message of Perses. Each query is then run once as a range query over `time_range` (default the past
hour; the step is chosen as for `perses_query_prometheus`). The result has:

- in the text content, read by the model: `{status, message, datasource, timeRange, step, displayedPanels, summaries,
  warnings}`, where each summary is a query with the statistics of its 8 highest series (`name` from the legend,
  `min`, `max`, `avg`, `last`, `minAt`, `maxAt`); a series that only has NaN or infinite samples has no statistics,
  with a warning;
- in the structured content: the same with the Perses definitions of the panels (`panels`) instead of
  `displayedPanels`, and the `scrapeInterval` of the datasource, used by the view and by chat UIs that render Perses
  panels.

When a query fails, a panel has no data (unless `allow_no_data`), or a `HeatMapChart` or `HistogramChart` gets no native
histogram, `status` is `error`, there are no panels and `message` tells the model what to fix.

#### MCP Apps

`perses_render_panels` follows the [MCP Apps](https://modelcontextprotocol.io/docs/extensions/apps) extension: its
`_meta.ui.resourceUri` is the view `ui://perses/panels.html`, which the clients that support MCP Apps (e.g. Claude,
ChatGPT, VS Code GitHub Copilot) display inline in the chat: the live panels, with a time range picker, a refresh
button and a fullscreen mode, in the theme and time zone of the client. The other clients only get the text content.

The view is a single HTML file, built from [`ui/mcp-apps`](https://github.com/perses/mcp-server/tree/main/ui/mcp-apps)
with the panel plugins of Perses and embedded in the server: the panels are those of the server release, which may
not be the versions of the plugins installed in Perses. It makes no network request: the Prometheus plugin of the
view gets its data from `perses_query_prometheus_data`, which runs an instant or range query through the Perses proxy
like the other Prometheus tools and returns the response of Prometheus in the structured content, and as JSON in the
text content for the hosts that only pass the content to the views (an error over 1 MiB: aggregate the query or
shorten the time range). This tool is only for the view (`_meta.ui.visibility: ["app"]`): it is not listed to the
clients that don't declare the `io.modelcontextprotocol/ui` extension, which would show it to the model.

Each query runs twice when the panels are first displayed: once by `perses_render_panels`, for the statistics of the
model, and once by the view, as the Prometheus plugin of Perses chooses the step of a query from the width of its panel
(at least the scrape interval of the datasource, as in Perses).
Changing the time range or refreshing only runs the queries of the view; the view then tells the model the new time
range (`ui/update-model-context`), as the statistics are those of the time range of the call. The automatic refresh
stops while the view is scrolled out of the chat.

The releases of the server (binaries and images) embed the view. A server built from the sources embeds it when
`npm` is installed (Node.js, see `ui/mcp-apps/.nvmrc`): otherwise `perses_render_panels` and
`perses_query_prometheus_data` are not registered, and the server logs a warning when it starts. To change the view,
edit `ui/mcp-apps` and run `make build`. To see the view in an MCP Apps host (Claude Desktop, VS
Code, the basic-host of ext-apps...), see
[Testing locally](https://github.com/perses/mcp-server/tree/main/ui/mcp-apps#testing-locally).

## Variables

Resources: `globalvariable` for the tools `*_global_variable*`, `variable` for the tools `*_project_variable*`.

| Tool                                  | Description                        | Required parameters        |
|---------------------------------------|------------------------------------|----------------------------|
| `perses_list_global_variables`        | List all global variables          | -                          |
| `perses_get_global_variable_by_name`  | Get a global variable by name      | `name`                     |
| `perses_create_global_variable`       | Create a new global variable       | `name`, `value`            |
| `perses_update_global_variable`       | Update an existing global variable | `name`, `value`            |
| `perses_delete_global_variable`       | Delete a global variable           | `name`                     |
| `perses_list_project_variables`       | List all variables of a project    | `project`                  |
| `perses_get_project_variable_by_name` | Get a project variable by name     | `project`, `name`          |
| `perses_create_project_variable`      | Create a variable in a project     | `project`, `name`          |
| `perses_update_project_variable`      | Update an existing project variable | `project`, `name`, `value` |
| `perses_delete_project_variable`      | Delete a variable from a project   | `project`, `name`          |

## Roles and role bindings

Resources: `globalrole`, `globalrolebinding`, `role` and `rolebinding`.

See [Authorization](https://perses.dev/docs/perses/latest/concepts/authorization/) for the meaning of the roles, the
role bindings, their actions, scopes and subjects.

| Tool                                      | Description                             | Required parameters                    |
|-------------------------------------------|-----------------------------------------|----------------------------------------|
| `perses_list_global_roles`                | List all global roles                   | -                                      |
| `perses_get_global_role_by_name`          | Get a global role by name               | `name`                                 |
| `perses_create_global_role`               | Create a new global role                | `name`, `actions`, `scopes`            |
| `perses_update_global_role`               | Update an existing global role          | `name`, `actions`, `scopes`            |
| `perses_delete_global_role`               | Delete a global role                    | `name`                                 |
| `perses_list_global_role_bindings`        | List all global role bindings           | -                                      |
| `perses_get_global_role_binding_by_name`  | Get a global role binding by name       | `name`                                 |
| `perses_create_global_role_binding`       | Create a new global role binding        | `name`, `role`, `subjects`             |
| `perses_update_global_role_binding`       | Update an existing global role binding  | `name`, `role`, `subjects`             |
| `perses_delete_global_role_binding`       | Delete a global role binding            | `name`                                 |
| `perses_list_project_roles`               | List all roles of a project             | `project`                              |
| `perses_get_project_role_by_name`         | Get a project role by name              | `project`, `name`                      |
| `perses_create_project_role`              | Create a new role in a project           | `project`, `name`, `actions`, `scopes` |
| `perses_update_project_role`              | Update an existing project role         | `project`, `name`, `actions`, `scopes` |
| `perses_delete_project_role`              | Delete a role from a project            | `project`, `name`                      |
| `perses_list_project_role_bindings`       | List all role bindings of a project     | `project`                              |
| `perses_get_project_role_binding_by_name` | Get a project role binding by name      | `project`, `name`                      |
| `perses_create_project_role_binding`      | Create a new role binding in a project  | `project`, `name`, `role`, `subjects`  |
| `perses_update_project_role_binding`      | Update an existing project role binding | `project`, `name`, `role`, `subjects`  |
| `perses_delete_project_role_binding`      | Delete a role binding from a project    | `project`, `name`                      |

## Plugins

Resource: `plugin`

| Tool                  | Description                              | Required parameters |
|-----------------------|------------------------------------------|---------------------|
| `perses_list_plugins` | List all the plugins installed in Perses | -                   |

## Schemas

Resource: `schema`

These read-only tools return raw [CUE](https://cuelang.org/) source from Perses v0.55 or later.

> [!NOTE]
> On Perses v0.55.x, shared types such as calculation, format and thresholds are referenced but not defined.
> [perses/perses#4738](https://github.com/perses/perses/pull/4738) fixes this but isn't released yet.

| Tool | Description | Required parameters |
|------|-------------|---------------------|
| `perses_list_plugin_schemas` | Get all installed plugin schemas | - |
| `perses_get_plugin_schema` | Get the latest installed schema of one plugin kind | `kind` |

Prefer the single-plugin tool when working on one panel, query, variable, annotation or datasource.

## Validation

Resource: `validate`

These tools validate resources without creating or modifying anything, and stay available in read-only mode.
They take resources as JSON objects and return `{valid: true}`, or `{valid: false, error}` when the resource is
invalid: `error` is the message of the Perses model when the resource cannot be decoded, or the message of the Perses
server when it rejects the resource (HTTP 400). When the resource could not be validated, e.g. because Perses is
unreachable or returns a server error, the tool returns the error of the Perses client unchanged as a tool error.

| Tool                        | Description                           | Required parameters |
|-----------------------------|---------------------------------------|---------------------|
| `perses_validate_dashboard` | Validate a dashboard                  | `dashboard`         |
| `perses_validate_variable`  | Validate a project or global variable | `variable`          |

`kind` is required on a variable and tells whether it is a project or a global one: `Variable` or `GlobalVariable`
(case-insensitive). Any other or missing kind returns `{valid: false, error}` without calling Perses.

## Grafana dashboards migration

Resource: `migrate`

| Tool                       | Description                                      | Required parameters | Optional parameters |
|----------------------------|--------------------------------------------------|---------------------|---------------------|
| `perses_migrate_dashboard` | Convert a Grafana dashboard into a Perses dashboard | `grafanaDashboard`  | `input`             |

- `grafanaDashboard`: the JSON of the Grafana dashboard, as a string.
- `input`: the values of the `__inputs` placeholders of the Grafana dashboard, for example
  `{"DS_PROMETHEUS": "my-datasource"}`.

The tool uses the [migration API](https://perses.dev/docs/perses/latest/api/migrate/) of Perses. The migrated
dashboard is returned to the LLM but **not saved**: use the tool `perses_create_dashboard` to save it.

When the LLM host supports [elicitation](https://modelcontextprotocol.io/specification/2025-06-18/client/elicitation),
the MCP server asks the user how to handle the datasources of the panels:

- keep the datasource references of the Grafana dashboard,
- or replace them with the default datasource of Perses.

The LLM can't make this choice on behalf of the user. When the LLM host doesn't support elicitation, the datasource
references of the Grafana dashboard are kept.
