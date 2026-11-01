# Dashboard tools: improvement report

Scope: the dashboard creation workflow of the MCP server (`perses_create_dashboard`, `perses_update_dashboard`,
`perses_validate_dashboard`) as experienced by an AI agent building a large (34-panel) dashboard. The work was done
iteratively (read → implement → build/lint/test → exercise against a real Perses → reflect), committed as
`1b163bb` on `ibakshay-schema-tools-self-review`, and consolidated into the schema and validation work of PR #113.
The dashboard validator is registered once under the `validate` resource. The work builds on the schema tools
(`perses_get_dashboard_schema`, `perses_list_plugin_schemas`, `perses_get_plugin_schema`, Perses client `Schema()` API,
Perses v0.55.1-pre). The two efforts complement each other: the schema tools tell the model *what* the server accepts,
the dashboard tools below make writing it cheap and report *where* it was rejected.

## a. Findings about the original design

| # | Finding | Impact on an agent |
|---|---------|--------------------|
| 1 | `dashboard` was a `string` parameter (JSON-as-string). | A 22 KB dashboard costs ~25 KB once escaped (+16 %), every quote is a `\"`, one bad escape means re-emitting everything. Agents fall back to `curl`. |
| 2 | Create/Update echoed the whole saved dashboard. | 22 KB of tokens per write that carry no information the agent did not already have. |
| 3 | `perses_get_dashboard_by_name` returned `nil, out, nil` from the go-sdk handler. | The SDK serialises `out` **twice** (as `structuredContent` and as a `TextContent` fallback): a 21.8 KB dashboard was 43.7 KB on the wire. |
| 4 | Validation errors were an opaque wrapped string: `error updating dashboard …: something wrong happened with the request to the API. Message: bad request: invalid panel panel_00: spec.visual.lineWidth: conflicting values "thick" and number …`. | The agent has to parse prose to find the field; the CUE path (`spec.visual.lineWidth`) is relative to the plugin, not to the dashboard. |
| 5 | All-or-nothing writes. Adding one panel = Get (43 KB) + re-send everything (25 KB) + echo (22 KB). | ~90 KB per incremental step; no way to validate a single panel. |
| 6 | Create retried after a transient failure → `409 Conflict`; no `dry_run`. | Needs separate validate → create → update round-trips. |
| 7 | `IdempotentHint: true` on Create (and Update). | Wrong for Create: the second POST 409s. Clients may auto-retry. |
| 8 | Tool descriptions assumed Perses knowledge (panel refs, layout items, 24-column grid, default datasource). | Models invent `spec.panels[]` arrays, forget layout items (panel silently not rendered), or set an unnecessary datasource. |
| 9 | The server does not reject dashboards that *look* wrong but are schema-valid: grid items beyond column 24, overlapping items, panels that no layout references, duplicate variable names. | The dashboard saves fine and renders broken; nothing tells the agent. |
| 10 | The Perses server has no optimistic locking (`PUT` always wins, version = previous + 1). | Concurrent editors silently overwrite each other. |

## b. Changes made

### New shared package `pkg/tools/internal/dashboardutil/` (new)

| File | Purpose |
|------|---------|
| `issue.go` | `Issue{path,message,hint}`, `Validation{valid,errors,warnings}`, `MarshalText` (compact JSON without HTML escaping so hints like `<key>` stay readable). |
| `decode.go` | `Decode(raw, project)` / `DecodeValue`: accepts an **object or a JSON string**, checks `kind`, `metadata.name`, `metadata.project`, turns `json.SyntaxError`/`UnmarshalTypeError` into an `Issue` with a JSON path (`malformed JSON at offset …`). |
| `lint.go` | Client-side lint for what the server accepts silently (finding 9): item `x+width > 24` / negative / zero sizes (errors), dangling `$ref` (error), duplicate variable names (error), overlapping items and never-displayed panels (warnings; orphans are aggregated into one warning). Handles `Grid` and `Tabs` layouts. |
| `servererror.go` | Parses the Perses 400 messages (`invalid panel K: <cue path>: msg`, `panel "K": invalid query n°N`, `invalid variable NAME`, `schema not found for plugin P`, echo-wrapped `code=400, message=…`, `no panel found for ref`) into Issues whose path is **absolute in the dashboard** (`spec.panels.K.spec.plugin.spec.visual.lineWidth`, `spec.panels.K.spec.queries[0].spec.plugin.spec.expr`, `spec.variables[i].spec.plugin.spec…`) and adds a hint pointing at the right schema tool. |
| `layout.go` | `EnsureLayout` (find by title / create a Grid group), `PlacePanel` (skyline auto-placement in the 24-column grid), `PlacePanelAt`, `RemovePanelItems`, `IsPlaced`, `IsValidKey`. |
| `summary.go` | `Summarize` (project, name, display, version, URL, counts) and `OutlineOf` (per-panel key/name/plugin/queries/placement, variables, layouts). `DashboardURL` builds `<perses>/projects/<p>/dashboards/<n>`. |
| `dashboardutil_test.go` | Unit tests for decode, lint, server-error parsing, placement and summary. |

### `pkg/tools/dashboard/` (rewritten / new)

| File | Change |
|------|--------|
| `dashboard.go` (modified) | `New(apiClient.ClientInterface)` (needs the top-level client for `Validate()` and the base URL). `persesModel` constant shared by all descriptions teaches the model: panel map keys ↔ `#/spec/panels/<key>` refs, a panel renders only when a layout item references it, 24-column grid, datasource may be omitted, kinds are case-sensitive (finding 8). List returns a compact `[{name, display, version, panels, updatedAt}]`. Get gains `summary: true` (outline) and `panels: [keys]` (only those panels + `missing`), and returns text only (finding 3). Shared `validate()` = server validation + lint. |
| `write.go` (new) | Create: `dashboard` is `object \| string` (`json.RawMessage`, `Types: ["object","string"]`); `if_exists: fail\|replace` (409 → Update); `dry_run`; `include_dashboard` (opt-in echo); `IdempotentHint: false`. Update: `expected_version`, `dry_run`, `include_dashboard`. Both return the short summary + `warnings`; validation failures are a tool error whose text is the structured `Validation` JSON. 409 message tells the agent about `if_exists=replace`. |
| `edit.go` (new) | Incremental tools implemented as Get → mutate in Go → validate → Update, all accepting `expected_version`, `dry_run`, `include_dashboard` and returning `changes[]`: `perses_set_dashboard_panels` (panels object-or-string; `layout` title creates the group when missing; `width`/`height` defaults 12×6; `placements` for explicit positions; existing placed panels keep their position; `kind` defaults to `Panel`; keys validated), `perses_remove_dashboard_panels` (also drops layout items), `perses_set_dashboard_variables` (replace by `spec.name` else append), `perses_remove_dashboard_variables`, `perses_set_dashboard_layout` (add / replace by `index` or `title` / `remove`). |
| `guide.go` (new) | MCP **resource** `perses://guides/dashboard-model` (`text/markdown`, 3.3 KB): worked example, rules the server does not enforce, incremental workflow, error format. The per-tool `persesModel` reminder shrank to ~330 B and points at the resource (descriptions: create 1273→966 B, set_panels 1342→1035 B, update 1078→771 B). Review question: "can this be gotten via a schema tool?" — no: the dashboard schema is `EncodeType(v1.Dashboard{})`, `Layout.Spec` is `any`, so grid items, the 24-column rule, the "render only if referenced" rule and default-datasource semantics are absent from it. |
| `dashboard_test.go` (new) | In-memory Perses (CRUD, 409 on duplicate POST, validation endpoint) driving every tool through a real MCP in-memory session; asserts compact output, no `structuredContent`, structured errors, placement coordinates, version checks. |

### Dashboard validation integration

Enhances the existing `perses_validate_dashboard` in `pkg/tools/validate/dashboard.go`: it accepts `object | string` and returns
`{valid, errors[{path,message,hint}], warnings}` using the same `dashboardutil` pipeline (decode → lint → server
validation via `client.Validate().Dashboard()`), so the validate tool and the write tools report identical errors.
The schema tools retain their v1 client and `schema` resource; the validator remains under `validate`, avoiding
duplicate tool registration and preserving resource filtering. `project` is optional when the dashboard already
contains `metadata.project`. The schema/validation integration tests use a fake Perses with the
`/api/validate/dashboards` route and validate tests were added (object input, absolute paths, dangling ref, warnings,
server error).

### Other

| File | Change |
|------|--------|
| `internal/permcp/server.go` | `dashboard.New(s.persesClient)` (top-level client instead of `V1()`); registers the guide resource unless the `dashboard` resource type is filtered out. |
| `README.md` | Dashboards and Schemas tool tables document the new tools/options and the response shape. |
| `go.mod` / `go.sum` | `github.com/stretchr/testify` added (used by `dashboard_test.go` and `dashboardutil_test.go`; the schema tests stay stdlib-only). |
| `docs/dev/dashboard-tool-improvement-report.md` | This report. |

The committed schema tools (`DashboardSchema`, `List`, `Get`, `kindPattern`, `textResult`) are untouched apart from the
client type change above.

## c. Measurements (real Perses 0.55.0-beta.4, 34-panel / 5-group dashboard, 21,717 B as JSON)

Wire bytes of the MCP `tools/call` arguments and result content (`structuredContent` counted since it is transmitted).
Old = binary built from the pre-change tree; New = this working tree. Driver: `files/bench/main.go` in the session folder.

| Step | Old request | Old response | New request | New response | Saving |
|------|------------:|-------------:|------------:|-------------:|-------:|
| Create the dashboard | 25,151 (escaped string) | 21,843 (full echo) | 21,761 (object) | 224 (summary + URL) | **46,994 → 21,985 B (−53 %)** |
| Inspect the dashboard | 54 | 43,686 (echoed twice) | 69 | 5,394 (`summary: true`) | **−88 %** (full get is now 21.8 KB, −50 %) |
| Add one panel | 25,151 + prior Get 43,686 | 21,843 | 623 (`set_dashboard_panels`) | 300 | **90,680 → 923 B (−99 %)** |
| Fix one invalid field | 25,137 | 441 (opaque prose) | 590 | 272 (`path` + `message` + `hint`) | **−97 %**, and actionable |

Error text, before:

```
error updating dashboard in project 'akshay-playground': something wrong happened with the request to the API.
Message: bad request: invalid panel panel_00: spec.visual.lineWidth: conflicting values "thick" and number (mismatched types string and number):
    ../etc/perses/plugins/TimeSeriesChart-0.14.0/schemas/time-series.cue:26:18
```

after (tool error, `isError: true`):

```json
{"valid":false,"errors":[{"path":"spec.panels.panel_00.spec.plugin.spec.visual.lineWidth",
  "message":"conflicting values \"thick\" and number (mismatched types string and number)",
  "hint":"use perses_get_plugin_schema with the panel plugin kind to see the allowed fields"}]}
```

End-to-end session (`files/e2e/main.go`: dry-run, create, 409, replace, 4 panels auto-placed into a new group,
3 deliberate errors — wrong type, wrong plugin case, wrong query field —, variables, summary, targeted get, layout
replace, removals, validate, delete): 17 calls, 8.2 KB of requests, 8.3 KB of responses in total. Every error came back
with an absolute path and a hint; the misspelled plugin kind `TimeseriesChart` produced
`the plugin "TimeseriesChart" is not installed on the Perses server` + `plugin kinds are case-sensitive`.

Tool-description overhead (sent once per session in `tools/list`): the 10 dashboard tools total ≈ 5.3 KB of
descriptions and ≈ 10 KB of input schemas, i.e. less than one old full-dashboard echo.

## d. Suggestions not implemented

1. **Server-side optimistic locking** (Perses): `PUT` should honour `If-Match`/`metadata.version` and return 412. The
   client-side `expected_version` check has a TOCTOU window between the Get and the Put; only the server can close it.
2. **Validate a single panel / variable without a dashboard.** The server only exposes `/api/validate/dashboards`; the
   incremental tools wrap the panel in the fetched dashboard instead. A server endpoint would make `dry_run` cheaper.
3. **`perses_set_dashboard_panels` on `Tabs` layouts.** Auto-placement targets `Grid` groups only (tabs are linted and
   summarised). Placing into a tab needs a `tab` selector; deferred until there is demand.
4. **Panel templates / `perses_render_panel`** (e.g. "time series for PromQL X"): would shrink requests further but
   duplicates plugin knowledge that changes with plugin versions; the schema tools already let the model look specs up.
5. **YAML input.** Would save quotes/braces but the JSON path reporting and the MCP schema typing become ambiguous;
   object input already removes the escaping problem.
6. **Reordering / resizing existing items** (`perses_move_dashboard_panel`): can be done today with `placements` in
   `set_dashboard_panels` (re-sending the panel) or by `set_dashboard_layout`; a dedicated tool is a nicety.
7. **Pagination of `perses_list_dashboards`** for projects with hundreds of dashboards (the compact entry is ~120 B).
8. **Upstream improvement to Perses error messages**: the CUE position lines (`../etc/perses/plugins/...cue:26:18`) are
   useless to API clients; the parser strips them, but the server could return structured errors itself.
9. **Resource filtering and read-only mode:** consolidation keeps `perses_validate_dashboard` under the existing
   `validate` resource. Registration tests cover resource isolation and filtering of the `set_*`/`remove_*` write tools.

## e. Commands

```sh
# build, vet, format, lint, unit tests (all green on this tree)
go build ./... && go vet ./...
gofmt -l .                           # must print nothing
golangci-lint run --timeout 5m       # 0 issues
go test ./...

make build                           # bin/perses-mcp-server

# live verification (Perses without auth on :8080 with project akshay-playground; the drivers live in the session
# artifacts folder ~/.copilot/session-state/<session>/files)
cd files/e2e  && go run . <worktree>/bin/perses-mcp-server <path-to>/perses-mcp-config.yaml
cd files/bench && go run . <old-or-new binary> perses-mcp-config.yaml old|new

# review
git status --short && git diff
```

Note: the original `localhost:8080` instance was no longer reachable during the exercise, so the live run used the
local Perses dev build (`perses-workspace/perses/bin/perses`, v0.55.0-beta.4, auth disabled, file database in the
session folder) seeded with project `akshay-playground` and a default `thanos-regional-query` Prometheus datasource.
The test dashboards were deleted at the end.
