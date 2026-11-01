# Tools

This page lists the tools provided by the Perses MCP Server. The tools are grouped by
[resource](./configuration.md#resources): use the setting `resources` to only register some of them.

> [!NOTE]
> In [read-only mode](./configuration.md#definition) (`read_only: true`), the tools creating, updating or deleting
> resources (`perses_create_*`, `perses_update_*`, `perses_delete_*`, `perses_set_*` and `perses_remove_*`) are not
> registered. Read and validation tools stay available.

Whatever the tools registered, they can only do what the identity configured in the MCP server is allowed to do in
Perses (see [Permissions](./overview.md#permissions)).

The server sends concise workflow instructions during MCP initialization, composed from the tools actually
registered after resource filtering and read-only mode. These cover compact dashboard reads, incremental editing,
plugin schemas and validation without requiring the client to read an MCP resource. The detailed dashboard guide
remains available at `perses://guides/dashboard-model` when the `dashboard` resource is enabled and the client supports
resources.

With `dashboard` enabled, the MCP `initialize` response advertises the `resources` capability.
A resource-capable client can discover the `text/markdown` guide with `resources/list`, then call
`resources/read` with `{"uri":"perses://guides/dashboard-model"}` and use the returned `contents[].text`
as context. The guide remains readable in read-only mode. Filtering out `dashboard` removes the guide,
its resource capability and its instruction reference.

This is an MCP resource, not a tool or an HTTP URL. Initialization instructions explicitly recommend
reading it only when the client supports MCP resources; clients without that support still receive the
essential dashboard rules inline. Capability advertisement and instructions do not guarantee that a
client exposes resources to a model, reads the guide, or follows it.

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
| `perses_get_dashboard_by_name` | Get an outline by default; use `summary: false` for complete JSON before read-modify-write, or `panels: [keys]` for those full panels regardless of `summary` | `project`, `name` |
| `perses_create_dashboard`      | Create a dashboard in a project            | `project`, `dashboard` |
| `perses_update_dashboard`      | Update an existing dashboard of a project  | `project`, `dashboard` |
| `perses_delete_dashboard`      | Delete a dashboard from a project          | `project`, `name`      |
| `perses_set_dashboard_panels` | Add, replace, move or resize panels, with automatic or explicit placement | `project`, `name`; at least one entry in `panels` or `placements` |
| `perses_remove_dashboard_panels` | Remove panels and their layout items | `project`, `name`, `panels` |
| `perses_set_dashboard_variables` | Add or replace variables by name | `project`, `name`, `variables` |
| `perses_remove_dashboard_variables` | Remove variables by name | `project`, `name`, `variables` |
| `perses_set_dashboard_layout` | Add, replace or remove a layout group | `project`, `name` |

The parameter `dashboard` is a dashboard in the [Perses format](https://perses.dev/docs/perses/latest/api/dashboard/).
Create and update accept a JSON object or a JSON string and validate before saving. Create supports
`if_exists: "replace"` to overwrite an existing dashboard. Writes return a compact summary; set
`include_dashboard: true` to include the saved dashboard. Use `dry_run: true` to validate without saving.

The incremental tools fetch the current dashboard, apply the requested changes, validate, and save it.
`expected_version` checks the version before editing to detect intervening changes; it is not an atomic
server-side lock, so a concurrent write after the check can still race. Layout items use a 24-column grid;
new panels default to width 12 and height 6. The MCP resource `perses://guides/dashboard-model` documents
the dashboard model and editing workflow.

For placements-only moves/resizing, omit `panels` (or send `{}`) and provide
`placements: {"cpu": {"x": 0, "y": 6, "width": 12, "height": 6}}`.
Placement keys must identify existing panels or panels supplied in the same call. Definitions are unchanged
when omitted, and invalid edits do not persist any changes. Explicit positions are reserved before automatic
placement. Existing panels without explicit placements keep their positions.

Use `order: ["b", "a", "c"]` to choose processing/auto-placement order: on an empty group with default 12x6
sizes, these panels are placed at `(0,0)`, `(12,0)` and `(0,6)`. Keys not listed are processed alphabetically.
Every order key must appear in `panels` or `placements`, without duplicates. Do not rely on JSON object
key order, including for JSON-string input. Use `perses_set_dashboard_layout` to replace a full group.

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

`perses_validate_dashboard` checks the dashboard configuration against installed plugin CUE schemas without saving it.
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

The response body is truncated to 1 MiB, to avoid exceeding the context of the LLM.

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

| Tool | Description | Required parameters |
|------|-------------|---------------------|
| `perses_get_dashboard_schema` | Get the complete dashboard schema, including installed plugins (large) | - |
| `perses_list_plugin_schemas` | Get all installed plugin schemas | - |
| `perses_get_plugin_schema` | Get one plugin kind's schema, optionally selecting `version` and `registry` | `kind` |

Prefer the single-plugin tool when working on one panel, query, variable, annotation or datasource.

## Validation

Resource: `validate`

These tools validate resource JSON against the installed plugin CUE schemas without creating or modifying anything.
They stay available in read-only mode. Dashboard validation returns `{valid, errors: [{path, message, hint}], warnings}`
with field-level errors and local layout checks; the other validators return `{"valid": true}` or
`{"valid": false, "errors": "..."}`.
Malformed JSON and structural validation failures also return `valid: false`; connection and unexpected server
errors are tool errors.

| Tool                               | Description                   | Required parameters |
|------------------------------------|-------------------------------|---------------------|
| `perses_validate_dashboard`         | Validate a dashboard          | `dashboard`         |
| `perses_validate_datasource`        | Validate a project datasource | `datasource`        |
| `perses_validate_global_datasource` | Validate a global datasource  | `datasource`        |
| `perses_validate_variable`          | Validate a project variable   | `variable`          |
| `perses_validate_global_variable`   | Validate a global variable    | `variable`          |

Dashboard validation accepts an object or JSON string, with optional `project` to fill `metadata.project`.
When `project` is omitted, the dashboard's own `metadata.project` is retained. The other validators accept
the full resource JSON as a string, including `kind`, `metadata`, and `spec`.
Validation does not persist the resource: creating or updating it requires the corresponding write tool.

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
