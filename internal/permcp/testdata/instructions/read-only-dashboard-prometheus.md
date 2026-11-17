Perses is an open-source observability dashboard tool. This server gives access to Perses resources.

To answer questions about monitored systems (metrics):
1. Datasource: use the same `datasource` for discovery and queries.
2. Before writing PromQL, check the metric and label names with `perses_list_prometheus_metric_names`, `perses_list_prometheus_label_names` and `perses_list_prometheus_label_values`, and the metric types with `perses_get_prometheus_metric_metadata`.
3. Query with `perses_query_prometheus`. For a past time range, pass the same `time_range` to `perses_list_prometheus_metric_names`, `perses_list_prometheus_label_names` and `perses_list_prometheus_label_values`.

The panels of existing dashboards (`perses_list_dashboards`, `perses_get_dashboard_by_name`) show which metrics and queries matter for a system.
