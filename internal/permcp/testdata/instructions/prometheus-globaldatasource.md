Perses is an open-source observability dashboard tool. This server gives access to Perses resources (projects, dashboards, datasources, variables).

To answer questions about monitored systems (metrics):
1. Datasource: use the same `datasource` for discovery and queries; `perses_list_global_datasources` list the other datasources.
2. Before writing PromQL, check the metric and label names with `perses_list_prometheus_metric_names`, `perses_list_prometheus_label_names` and `perses_list_prometheus_label_values`, and the metric types with `perses_get_prometheus_metric_metadata`.
3. Query with `perses_query_prometheus`, not `perses_query_global_datasource`, which return raw responses. For a past time range, pass the same `time_range` to the discovery tools.
