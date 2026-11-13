// Copyright The Perses Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package prometheus exposes compact tools to discover and query the metrics of the Prometheus datasources of
// Perses, through the Perses datasource proxy.
package prometheus

import (
	"context"
	"fmt"
	"time"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/perses/mcp-server/pkg/tools"
	"github.com/perses/mcp-server/pkg/tools/resource"
	apiClient "github.com/perses/perses/pkg/client/api/v1"
)

const (
	ListMetricNamesToolName   = "perses_list_prometheus_metric_names"
	ListLabelNamesToolName    = "perses_list_prometheus_label_names"
	ListLabelValuesToolName   = "perses_list_prometheus_label_values"
	GetMetricMetadataToolName = "perses_get_prometheus_metric_metadata"
	QueryToolName             = "perses_query_prometheus"
)

type prometheus struct {
	client   apiClient.ClientInterface
	resolver *resolver
	now      func() time.Time
}

func New(client apiClient.ClientInterface) resource.Resource {
	return &prometheus{client: client, resolver: newResolver(client), now: time.Now}
}

func (p *prometheus) GetTools() []*tools.Tool {
	return []*tools.Tool{
		p.ListMetricNames(),
		p.ListLabelNames(),
		p.ListLabelValues(),
		p.GetMetricMetadata(),
		p.Query(),
	}
}

// Create is not supported for Prometheus datasources
func (p *prometheus) Create() *tools.Tool { return nil }

// Update is not supported for Prometheus datasources
func (p *prometheus) Update() *tools.Tool { return nil }

// Delete is not supported for Prometheus datasources
func (p *prometheus) Delete() *tools.Tool { return nil }

// List is not used: the listing tools are ListMetricNames, ListLabelNames and ListLabelValues
func (p *prometheus) List() *tools.Tool { return nil }

// Get is not used: the metadata tool is GetMetricMetadata
func (p *prometheus) Get() *tools.Tool { return nil }

func (p *prometheus) ListMetricNames() *tools.Tool {
	tool := &mcp.Tool{
		Name: ListMetricNamesToolName,
		Description: "List the metric names of a Prometheus datasource. Use it before writing PromQL: never guess metric names. " +
			"Narrow it with `search` (e.g. \"memory\") and/or `match` (e.g. \"{job=\\\"prometheus\\\"}\"); " +
			"`include_metadata: true` adds the type (counter, gauge...) and help of each metric.",
		Annotations: readOnlyAnnotations("Lists Prometheus metric names"),
		InputSchema: objectSchema(map[string]*jsonschema.Schema{
			"match":      matchSchema(),
			"search":     searchSchema(),
			"time_range": discoveryTimeRangeSchema(),
			"limit": limitSchema(fmt.Sprintf("Maximum number of metrics returned (default %d, max %d; with include_metadata default %d, max %d).",
				defaultMetricsLimit, maxListLimit, defaultMetadataMetricsLimit, maxMetadataMetricsLimit), maxListLimit),
			"include_metadata": {
				Type:        tools.SchemaTypeBoolean,
				Description: "Add the type and help (shortened) of each returned metric. Defaults to false.",
			},
		}),
	}
	handler := func(ctx context.Context, _ *mcp.CallToolRequest, input ListMetricNamesInput) (*mcp.CallToolResult, any, error) { //nolint:unparam
		output, err := p.listMetricNames(ctx, input)
		if err != nil {
			return nil, nil, err
		}
		return jsonResult(output)
	}
	return readOnlyTool(tool, func(server *mcp.Server) { mcp.AddTool(server, tool, handler) })
}

func (p *prometheus) ListLabelNames() *tools.Tool {
	tool := &mcp.Tool{
		Name: ListLabelNamesToolName,
		Description: "List the label names of a Prometheus datasource, e.g. to know how to group or filter a metric. Pass " +
			"`match` to get the labels of specific series, e.g. \"process_cpu_seconds_total\".",
		Annotations: readOnlyAnnotations("Lists Prometheus label names"),
		InputSchema: objectSchema(map[string]*jsonschema.Schema{
			"match":      matchSchema(),
			"time_range": discoveryTimeRangeSchema(),
			"limit":      limitSchema(fmt.Sprintf("Maximum number of labels returned (default %d, max %d).", defaultLabelsLimit, maxListLimit), maxListLimit),
		}),
	}
	handler := func(ctx context.Context, _ *mcp.CallToolRequest, input ListLabelNamesInput) (*mcp.CallToolResult, any, error) { //nolint:unparam
		output, err := p.listLabelNames(ctx, input)
		if err != nil {
			return nil, nil, err
		}
		return jsonResult(output)
	}
	return readOnlyTool(tool, func(server *mcp.Server) { mcp.AddTool(server, tool, handler) })
}

func (p *prometheus) ListLabelValues() *tools.Tool {
	tool := &mcp.Tool{
		Name: ListLabelValuesToolName,
		Description: "List the values of a label of a Prometheus datasource, e.g. the jobs (`label: \"job\"`) or the instances " +
			"of a job (`label: \"instance\", match: \"{job=\\\"node\\\"}\"`). Use it to write exact label matchers.",
		Annotations: readOnlyAnnotations("Lists Prometheus label values"),
		InputSchema: objectSchema(map[string]*jsonschema.Schema{
			"label": {
				Type: tools.SchemaTypeString, MinLength: new(1),
				Description: "Label name, e.g. \"job\", or a UTF-8 label name of Prometheus 3, e.g. \"http.method\".",
			},
			"match":      matchSchema(),
			"search":     searchSchema(),
			"time_range": discoveryTimeRangeSchema(),
			"limit":      limitSchema(fmt.Sprintf("Maximum number of values returned (default %d, max %d).", defaultValuesLimit, maxListLimit), maxListLimit),
		}, "label"),
	}
	handler := func(ctx context.Context, _ *mcp.CallToolRequest, input ListLabelValuesInput) (*mcp.CallToolResult, any, error) { //nolint:unparam
		output, err := p.listLabelValues(ctx, input)
		if err != nil {
			return nil, nil, err
		}
		return jsonResult(output)
	}
	return readOnlyTool(tool, func(server *mcp.Server) { mcp.AddTool(server, tool, handler) })
}

func (p *prometheus) GetMetricMetadata() *tools.Tool {
	tool := &mcp.Tool{
		Name: GetMetricMetadataToolName,
		Description: "Get the type (counter, gauge, histogram, summary), help and unit of Prometheus metrics, e.g. " +
			"`metrics: [\"go_memstats_heap_inuse_bytes\"]`. Use it to pick the right PromQL: rate() on counters, " +
			"histogram_quantile() on histogram buckets, units of the values.",
		Annotations: readOnlyAnnotations("Gets Prometheus metric metadata"),
		InputSchema: objectSchema(map[string]*jsonschema.Schema{
			"metrics": {
				Type:     tools.SchemaTypeArray,
				Items:    &jsonschema.Schema{Type: tools.SchemaTypeString, MinLength: new(1)},
				MinItems: new(1), MaxItems: new(maxMetadataMetrics),
				Description: fmt.Sprintf("Metric names (1 to %d). The series of histograms, summaries, counters and info "+
					"metrics (_bucket, _count, _sum, _gcount, _gsum, _total, _info) are also looked up under their family "+
					"name.", maxMetadataMetrics),
			},
		}, "metrics"),
	}
	handler := func(ctx context.Context, _ *mcp.CallToolRequest, input GetMetricMetadataInput) (*mcp.CallToolResult, any, error) { //nolint:unparam
		output, err := p.getMetricMetadata(ctx, input)
		if err != nil {
			return nil, nil, err
		}
		return jsonResult(output)
	}
	return readOnlyTool(tool, func(server *mcp.Server) { mcp.AddTool(server, tool, handler) })
}

func (p *prometheus) Query() *tools.Tool {
	tool := &mcp.Tool{
		Name: QueryToolName,
		Description: "Run a PromQL query and get a compact result. " +
			"type \"instant\" (default) returns the value of each series at `time` (default now), sorted by value, e.g. " +
			"`query: \"topk(5, sum by (handler) (rate(prometheus_http_requests_total[5m])))\"`. type \"range\" returns " +
			"per-series statistics over `time_range` (min, max, avg, last and when min/max happened), never raw points. " +
			"Aggregate (sum by, topk) to keep results small; labels shared by all series are returned once in " +
			"commonLabels. Values keep 4 significant digits, and all the digits of their integer part. " +
			"Errors of Prometheus (e.g. bad_data: parse error) are returned unchanged.",
		Annotations: readOnlyAnnotations("Queries a Prometheus datasource"),
		InputSchema: objectSchema(map[string]*jsonschema.Schema{
			"query": {Type: tools.SchemaTypeString, MinLength: new(1), Description: "PromQL expression. Use rate() on counters."},
			"type": {
				Type: tools.SchemaTypeString, Enum: []any{instantQuery, rangeQuery},
				Description: "instant (default): values at `time`. range: statistics over `time_range`.",
			},
			"time":       {Type: tools.SchemaTypeString, Description: "Instant queries only: evaluation time (RFC 3339). Defaults to now."},
			"time_range": timeRangeSchema("Range queries only: time range of the statistics. Defaults to the past hour."),
			"step": {
				Type: tools.SchemaTypeString,
				Description: "Range queries only: resolution, e.g. \"1m\". Defaults to about 500 points, at least the scrape " +
					"interval of the datasource (1m when not set). Raised if needed to stay under 11000 points.",
			},
			"limit": limitSchema(fmt.Sprintf("Maximum number of series returned, highest values first (default %d, max %d).",
				defaultSeriesLimit, maxSeriesLimit), maxSeriesLimit),
			"time_zone": {Type: tools.SchemaTypeString, Description: "IANA time zone of minAt/maxAt, e.g. \"Europe/Berlin\". Defaults to UTC."},
		}, "query"),
	}
	handler := func(ctx context.Context, _ *mcp.CallToolRequest, input QueryInput) (*mcp.CallToolResult, any, error) { //nolint:unparam
		output, err := p.query(ctx, input)
		if err != nil {
			return nil, nil, err
		}
		return jsonResult(output)
	}
	return readOnlyTool(tool, func(server *mcp.Server) { mcp.AddTool(server, tool, handler) })
}

func readOnlyTool(tool *mcp.Tool, register func(server *mcp.Server)) *tools.Tool {
	return &tools.Tool{
		MCPTool:      tool,
		IsWriteTool:  false,
		ResourceType: tools.PrometheusResource,
		RegisterWith: register,
	}
}

func readOnlyAnnotations(title string) *mcp.ToolAnnotations {
	return &mcp.ToolAnnotations{
		Title:           title,
		ReadOnlyHint:    true,
		DestructiveHint: new(false),
		IdempotentHint:  true,
		OpenWorldHint:   new(false),
	}
}

func jsonResult(output any) (*mcp.CallToolResult, any, error) {
	result, err := tools.JSONResult(output)
	if err != nil {
		return nil, nil, fmt.Errorf("error marshalling the result: %w", err)
	}
	return result, nil, nil
}

// objectSchema returns the input schema of a tool: the properties and the optional datasource.
func objectSchema(properties map[string]*jsonschema.Schema, required ...string) *jsonschema.Schema {
	properties["datasource"] = &jsonschema.Schema{
		Type: tools.SchemaTypeObject,
		Description: "The Prometheus datasource to query. Omit it (or omit name) to use the default Prometheus " +
			"datasource: the project's default when project is set, else the global default.",
		Properties: map[string]*jsonschema.Schema{
			"name": {
				Type: tools.SchemaTypeString, MinLength: new(1), MaxLength: new(75), Pattern: tools.PatternResourceName,
				Description: "Datasource name, e.g. \"prometheus\".",
			},
			"project": {
				Type: tools.SchemaTypeString, MinLength: new(1), MaxLength: new(75), Pattern: tools.PatternResourceName,
				Description: "Project of a project datasource. Omit for a global datasource.",
			},
		},
		AdditionalProperties: &jsonschema.Schema{Not: &jsonschema.Schema{}},
	}
	return &jsonschema.Schema{
		Type:                 tools.SchemaTypeObject,
		Properties:           properties,
		Required:             required,
		AdditionalProperties: &jsonschema.Schema{Not: &jsonschema.Schema{}},
	}
}

func matchSchema() *jsonschema.Schema {
	selector := func() *jsonschema.Schema { return &jsonschema.Schema{Type: tools.SchemaTypeString, MinLength: new(1)} }
	return &jsonschema.Schema{
		Description: "Series selector, or list of series selectors OR-ed together, restricting the result to the " +
			"matching series, e.g. \"{job=\\\"prometheus\\\"}\" or [\"up\", \"process_cpu_seconds_total\"].",
		AnyOf: []*jsonschema.Schema{
			selector(),
			{Type: tools.SchemaTypeArray, Items: selector(), MaxItems: new(10)},
		},
	}
}

func searchSchema() *jsonschema.Schema {
	return &jsonschema.Schema{Type: tools.SchemaTypeString, Description: "Case-insensitive substring filter, e.g. \"memory\"."}
}

func limitSchema(description string, maxLimit int) *jsonschema.Schema {
	return &jsonschema.Schema{Type: "integer", Minimum: new(1.0), Maximum: new(float64(maxLimit)), Description: description}
}

func timeRangeSchema(description string) *jsonschema.Schema {
	return &jsonschema.Schema{
		Type:        tools.SchemaTypeObject,
		Description: description,
		Properties: map[string]*jsonschema.Schema{
			"past_duration": {
				Type:        tools.SchemaTypeString,
				Description: "Relative range ending now: \"15m\", \"1h\" (default), \"6h\", \"24h\", \"7d\"...",
			},
			"start": {Type: tools.SchemaTypeString, Description: "Absolute range start (RFC 3339). Do not combine with past_duration."},
			"end":   {Type: tools.SchemaTypeString, Description: "Absolute range end (RFC 3339), defaults to now. Requires start."},
		},
		AdditionalProperties: &jsonschema.Schema{Not: &jsonschema.Schema{}},
	}
}

func discoveryTimeRangeSchema() *jsonschema.Schema {
	return timeRangeSchema("Time range of the series to consider. Defaults to the past hour. Prometheus applies it " +
		"approximately (by storage block): the list may include series without samples in it, e.g. series that stopped " +
		"a few hours ago.")
}
