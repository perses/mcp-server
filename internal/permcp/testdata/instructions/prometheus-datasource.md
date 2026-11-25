Perses is an open-source observability dashboard tool. This server gives access to Perses resources.

To answer questions about monitored systems (metrics):
1. Datasource: use the same `datasource` for discovery and queries; find the other datasources with `perses_list_project_datasources`.
2. Before writing PromQL, check the metric and label names with `perses_list_prometheus_metric_names`, `perses_list_prometheus_label_names` and `perses_list_prometheus_label_values`, and the metric types with `perses_get_prometheus_metric_metadata`.
3. Query with `perses_query_prometheus`, not the raw datasource proxy `perses_query_project_datasource`. For a past time range, pass the same `time_range` to `perses_list_prometheus_metric_names`, `perses_list_prometheus_label_names` and `perses_list_prometheus_label_values`.
4. When a chart or a table helps the user, show the data with `perses_render_panels`: it returns statistics of the series, and clients that support MCP Apps display the panels.
