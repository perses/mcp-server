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

package prometheus

import (
	"context"
	"fmt"
	"time"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/perses/mcp-server/pkg/mcpapp"
	"github.com/perses/mcp-server/pkg/tools"
	"github.com/perses/mcp-server/pkg/tools/resource"
	apiclient "github.com/perses/perses/pkg/client/api"
	v1 "github.com/perses/perses/pkg/model/api/v1"
	"github.com/sirupsen/logrus"
)

const (
	RenderPanelsToolName = "perses_render_panels"
	QueryDataToolName    = "perses_query_prometheus_data"
	// PanelsViewURI is the URI of the view of perses_render_panels, which MCP Apps hosts render inline.
	PanelsViewURI = "ui://perses/panels.html"
)

// panels exposes perses_render_panels and the tool that its view calls to query the data.
type panels struct {
	*prometheus
	// view is the HTML of the view of perses_render_panels.
	view      string
	installed *installedPanels
	// validate validates a dashboard with the Perses validation API.
	validate func(*v1.Dashboard) error
}

// NewPanels returns the tools that display Prometheus data as Perses panels in the given view, the HTML built from
// ui/mcp-apps (ui.PanelsHTML). It has no tools when the view is empty, i.e. when the server is built without it.
func NewPanels(client apiclient.ClientInterface, view string) resource.Resource {
	v1Client := client.V1()
	return &panels{
		prometheus: New(v1Client).(*prometheus),
		view:       view,
		installed:  &installedPanels{list: v1Client.Plugin().List, now: time.Now},
		validate:   client.Validate().Dashboard,
	}
}

func (p *panels) GetTools() []*tools.Tool {
	if p.view == "" {
		return nil
	}
	return []*tools.Tool{p.RenderPanels(), p.QueryData()}
}

func (p *panels) Create() *tools.Tool { return nil }
func (p *panels) Update() *tools.Tool { return nil }
func (p *panels) Delete() *tools.Tool { return nil }
func (p *panels) List() *tools.Tool   { return nil }
func (p *panels) Get() *tools.Tool    { return nil }

func (p *panels) RenderPanels() *tools.Tool {
	tool := &mcp.Tool{
		Name: RenderPanelsToolName,
		Description: "Show Prometheus data to the user as Perses panels (charts, stats, gauges, tables...). Use it when " +
			"a chart or a table helps answer a question; group related panels in one call. Clients that support MCP " +
			"Apps display the live panels; the result always has per-series statistics over the time range (min, " +
			"max, avg, last, when min/max happened; the highest series first) to ground the answer. Each query is " +
			"run once as a range query. If status is \"error\" (failed query, or no data), nothing is displayed: " +
			"fix the queries and call again. Write the queries as for perses_query_prometheus.",
		Annotations: readOnlyAnnotations("Shows Perses panels of Prometheus queries"),
		InputSchema: renderPanelsInputSchema(),
		OutputSchema: &jsonschema.Schema{
			Type: tools.SchemaTypeObject,
			Properties: map[string]*jsonschema.Schema{
				"status":     {Type: tools.SchemaTypeString, Enum: []any{panelsStatusOK, panelsStatusError}},
				"message":    {Type: tools.SchemaTypeString},
				"datasource": {Type: tools.SchemaTypeObject},
				"timeRange":  {Type: tools.SchemaTypeObject},
				"step":       {Type: tools.SchemaTypeString},
				"panels":     {Type: tools.SchemaTypeArray, Items: &jsonschema.Schema{Type: tools.SchemaTypeObject}},
				"summaries":  {Type: tools.SchemaTypeArray, Items: &jsonschema.Schema{Type: tools.SchemaTypeObject}},
				"warnings":   {Type: tools.SchemaTypeArray, Items: &jsonschema.Schema{Type: tools.SchemaTypeString}},
			},
			Required: []string{"status", "datasource", "timeRange", "panels", "summaries"},
		},
		Meta: mcpapp.ToolMeta(PanelsViewURI),
	}
	handler := func(ctx context.Context, _ *mcp.CallToolRequest, input RenderPanelsInput) (*mcp.CallToolResult, *RenderPanelsOutput, error) {
		output, err := p.render(ctx, input)
		if err != nil {
			return nil, nil, err
		}
		logrus.WithFields(logrus.Fields{
			"datasource": output.Datasource.Name,
			"project":    output.Datasource.Project,
			"panels":     len(input.Panels),
			"status":     output.Status,
		}).Debug("rendered panels")
		// The text is for the model, the structured content (with the panel definitions) for the views.
		result, err := tools.JSONResult(output.text())
		if err != nil {
			return nil, nil, fmt.Errorf("error marshalling the result: %w", err)
		}
		return result, output, nil
	}
	return readOnlyTool(tool, func(server *mcp.Server) {
		mcp.AddTool(server, tool, handler)
		server.AddResource(panelsViewResource(), p.readView)
	})
}

func (p *panels) QueryData() *tools.Tool {
	tool := &mcp.Tool{
		Name: QueryDataToolName,
		Description: "Called by the view of perses_render_panels (MCP Apps) to load the data of its panels: runs a " +
			"Prometheus instant query (time) or range query (start, end, step) and returns the response of the " +
			"Prometheus HTTP API in the structured content.",
		Annotations: readOnlyAnnotations("Queries Prometheus for the panels view"),
		InputSchema: objectSchema(map[string]*jsonschema.Schema{
			"query": {Type: tools.SchemaTypeString, MinLength: new(1), Description: "PromQL expression."},
			"time":  {Type: tools.SchemaTypeString, Description: "Instant query: evaluation time (Unix seconds or RFC 3339). Defaults to now."},
			"start": {Type: tools.SchemaTypeString, Description: "Range query: start (Unix seconds or RFC 3339)."},
			"end":   {Type: tools.SchemaTypeString, Description: "Range query: end (Unix seconds or RFC 3339)."},
			"step":  {Type: tools.SchemaTypeString, Description: "Range query: resolution (seconds or duration, e.g. \"15s\")."},
		}, "query"),
		Meta: mcpapp.ToolMeta("", mcpapp.VisibilityApp),
	}
	handler := func(ctx context.Context, _ *mcp.CallToolRequest, input QueryDataInput) (*mcp.CallToolResult, map[string]any, error) {
		// Calls are not restricted to the clients that declare MCP Apps: some hosts render the views without
		// declaring the extension (e.g. the basic-host of ext-apps), and the tool reads no more than
		// perses_query_prometheus.
		output, err := p.queryData(ctx, input)
		if err != nil {
			return nil, nil, err
		}
		// The data is only in the structured content, which MCP Apps defines for the views: the tool is app-only, so
		// only MCP Apps hosts call it, and they support the structured content. Repeating it as JSON in the text
		// content would double the size of the result.
		data, _ := output["data"].(map[string]any)
		return tools.TextResult(fmt.Sprintf("Prometheus %s result in the structured content.", data["resultType"])), output, nil
	}
	return readOnlyTool(tool, func(server *mcp.Server) { mcp.AddTool(server, tool, handler) })
}

func panelsViewResource() *mcp.Resource {
	return &mcp.Resource{
		URI:         PanelsViewURI,
		Name:        "perses-panels",
		Title:       "Perses panels",
		Description: "Interactive view of perses_render_panels: the Perses panels of the result, with a time range picker.",
		MIMEType:    mcpapp.ResourceMIMEType,
		Meta:        panelsViewMeta(),
	}
}

// panelsViewMeta is the _meta of the view: the view makes no network request (the data comes from the tools), and
// draws its own background.
func panelsViewMeta() mcp.Meta {
	return mcp.Meta{"ui": map[string]any{
		"csp":           map[string]any{"connectDomains": []string{}, "resourceDomains": []string{}},
		"prefersBorder": true,
	}}
}

func (p *panels) readView(_ context.Context, _ *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{{
		URI:      PanelsViewURI,
		MIMEType: mcpapp.ResourceMIMEType,
		Text:     p.view,
		Meta:     panelsViewMeta(),
	}}}, nil
}

func renderPanelsInputSchema() *jsonschema.Schema {
	query := &jsonschema.Schema{
		Type: tools.SchemaTypeObject,
		Properties: map[string]*jsonschema.Schema{
			"promql": {Type: tools.SchemaTypeString, MinLength: new(1), Description: "PromQL expression. Use rate() on counters."},
			"legend": {
				Type:        tools.SchemaTypeString,
				Description: "Series name, with label placeholders, e.g. \"{{instance}}\" or \"{{method}} {{code}}\". Defaults to the labels.",
			},
		},
		Required:             []string{"promql"},
		AdditionalProperties: &jsonschema.Schema{Not: &jsonschema.Schema{}},
	}
	kinds := make([]any, 0, len(panelKinds))
	for _, kind := range panelKindNames() {
		kinds = append(kinds, kind)
	}
	unitValues := make([]any, 0, len(units))
	for _, unit := range units {
		unitValues = append(unitValues, unit)
	}
	panel := &jsonschema.Schema{
		Type: tools.SchemaTypeObject,
		Properties: map[string]*jsonschema.Schema{
			"title":       {Type: tools.SchemaTypeString, MinLength: new(1), MaxLength: new(maxPanelTitle), Description: "Panel title."},
			"description": {Type: tools.SchemaTypeString, MaxLength: new(maxPanelDesc), Description: "Panel description."},
			"kind": {
				Type: tools.SchemaTypeString, Enum: kinds,
				Description: "Panel plugin. TimeSeriesChart (default): values over time. StatChart: one current " +
					"value with a sparkline. GaugeChart: a current value against a maximum (set spec.max unless the " +
					"unit is a percent). BarChart and PieChart: series compared by their last value. Table and " +
					"TimeSeriesTable: series as rows. StatusHistoryChart: states over time. HeatMapChart and " +
					"HistogramChart: native histograms only (e.g. sum(rate(x[5m])) of a native histogram x), not " +
					"classic _bucket series.",
			},
			"unit": {
				Type: tools.SchemaTypeString, Enum: unitValues,
				Description: "Unit of the values (of the buckets for HeatMapChart and HistogramChart), e.g. bytes, " +
					"bytes/sec, seconds, percent (0-100), percent-decimal (0-1), requests/sec. Not for Table, " +
					"TimeSeriesTable and StatusHistoryChart.",
			},
			"queries": {
				Type: tools.SchemaTypeArray, Items: query, MinItems: new(1), MaxItems: new(maxPanelQueries),
				Description: fmt.Sprintf("1 to %d queries.", maxPanelQueries),
			},
			"spec": {
				Type: tools.SchemaTypeObject,
				Description: "Optional plugin spec, merged over the defaults of the kind and validated against the " +
					"plugin schema of Perses (perses_get_plugin_schema), e.g. {\"thresholds\": {\"steps\": " +
					"[{\"value\": 80, \"color\": \"#e5484d\"}]}} or {\"visual\": {\"display\": \"bar\", \"stack\": " +
					"\"all\"}}. Usually omitted.",
			},
		},
		Required:             []string{"title", "queries"},
		AdditionalProperties: &jsonschema.Schema{Not: &jsonschema.Schema{}},
	}
	return objectSchema(map[string]*jsonschema.Schema{
		"time_range": timeRangeSchema("Time range of the panels and statistics. Defaults to the past hour."),
		"time_zone":  {Type: tools.SchemaTypeString, Description: "IANA time zone of minAt/maxAt, e.g. \"Europe/Berlin\". Defaults to UTC."},
		"allow_no_data": {
			Type:        tools.SchemaTypeBoolean,
			Description: "Display panels whose queries return no series. Defaults to false: such panels are an error.",
		},
		"panels": {
			Type: tools.SchemaTypeArray, Items: panel, MinItems: new(1), MaxItems: new(maxPanels),
			Description: fmt.Sprintf("1 to %d panels, displayed in this order.", maxPanels),
		},
	}, "panels")
}
