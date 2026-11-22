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
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/perses/perses/pkg/client/perseshttp"
	v1 "github.com/perses/perses/pkg/model/api/v1"
	"github.com/perses/spec/go/dashboard"
	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

const (
	maxPanels        = 6
	maxPanelQueries  = 5
	maxPanelTitle    = 80
	maxPanelDesc     = 200
	maxSummarySeries = 8
	// maxParallelQueries caps the queries run at the same time by a call.
	maxParallelQueries = 6
	// maxValidationErrorLength caps the validation errors of Perses.
	maxValidationErrorLength = 1000
	panelsStatusOK           = "ok"
	panelsStatusError        = "error"
	// validationPanelKey is the key of the panels in the dashboard sent to the Perses validation, e.g. "mcp_panel_0".
	validationPanelKey = "mcp_panel_"
)

// legendPlaceholder is a label placeholder of a legend, e.g. "{{instance}}", as in the Perses series name format.
var legendPlaceholder = regexp.MustCompile(`\{\{\s*([^{}\s]+)\s*\}\}`)

type RenderPanelsInput struct {
	Datasource  *DatasourceRef  `json:"datasource,omitempty"`
	TimeRange   *TimeRangeInput `json:"time_range,omitempty"`
	TimeZone    string          `json:"time_zone,omitempty"`
	AllowNoData bool            `json:"allow_no_data,omitempty"`
	Panels      []PanelInput    `json:"panels"`
}

type PanelInput struct {
	Title       string            `json:"title"`
	Description string            `json:"description,omitempty"`
	Kind        string            `json:"kind,omitempty"`
	Unit        string            `json:"unit,omitempty"`
	Queries     []PanelQueryInput `json:"queries"`
	Spec        map[string]any    `json:"spec,omitempty"`
}

type PanelQueryInput struct {
	PromQL string `json:"promql"`
	Legend string `json:"legend,omitempty"`
}

// seriesSummary is the statistics of a series of a panel query, named by the legend of the query.
type seriesSummary struct {
	Name string `json:"name"`
	stats
}

// querySummary is the result of a panel query. PanelIndex is the index of its panel in the input, Panel its title.
type querySummary struct {
	PanelIndex  int             `json:"panelIndex"`
	Panel       string          `json:"panel"`
	PromQL      string          `json:"promql"`
	SeriesCount int             `json:"seriesCount"`
	Series      []seriesSummary `json:"series"`
	// HistogramSeries is the number of native histogram series, which have no statistics.
	HistogramSeries int    `json:"histogramSeries,omitempty"`
	Error           string `json:"error,omitempty"`
}

// RenderPanelsOutput is the structured content of perses_render_panels, read by its view and by chat UIs (PersAI's
// ShowPanelsOutput): keep the field names.
type RenderPanelsOutput struct {
	Status     string             `json:"status"`
	Message    string             `json:"message,omitempty"`
	Datasource DatasourceRef      `json:"datasource"`
	TimeRange  timeRangeOutput    `json:"timeRange"`
	Step       string             `json:"step"`
	Panels     []*dashboard.Panel `json:"panels"`
	Summaries  []querySummary     `json:"summaries"`
	Warnings   []string           `json:"warnings,omitempty"`
}

// renderPanelsText is the text content of perses_render_panels, read by the model: the structured content without
// the panel definitions, which the model wrote and which only the views use.
type renderPanelsText struct {
	Status          string          `json:"status"`
	Message         string          `json:"message,omitempty"`
	Datasource      DatasourceRef   `json:"datasource"`
	TimeRange       timeRangeOutput `json:"timeRange"`
	Step            string          `json:"step"`
	DisplayedPanels int             `json:"displayedPanels"`
	Summaries       []querySummary  `json:"summaries"`
	Warnings        []string        `json:"warnings,omitempty"`
}

func (o *RenderPanelsOutput) text() renderPanelsText {
	return renderPanelsText{
		Status:          o.Status,
		Message:         o.Message,
		Datasource:      o.Datasource,
		TimeRange:       o.TimeRange,
		Step:            o.Step,
		DisplayedPanels: len(o.Panels),
		Summaries:       o.Summaries,
		Warnings:        o.Warnings,
	}
}

func (p *panels) render(ctx context.Context, input RenderPanelsInput) (*RenderPanelsOutput, error) {
	location, err := resolveLocation(input.TimeZone)
	if err != nil {
		return nil, err
	}
	start, end, timeRange, err := resolveTimeRange(input.TimeRange, p.now())
	if err != nil {
		return nil, err
	}
	var warnings []string
	installed, err := p.installed.get()
	if err != nil {
		logrus.WithError(err).Warn("error listing the plugins of Perses: the panel kinds are not checked against them")
		warnings = append(warnings, "the plugins installed in Perses could not be listed: the panel kinds were not checked against them")
	}
	kinds, err := checkPanels(input.Panels, installed)
	if err != nil {
		return nil, err
	}

	datasource, err := p.resolver.resolve(input.Datasource)
	if err != nil {
		return nil, err
	}
	panelDefinitions := make([]*dashboard.Panel, len(input.Panels))
	for i, panel := range input.Panels {
		panelDefinitions[i] = buildPanel(panel, kinds[i], datasource.ref)
	}
	if err = p.validatePanels(panelDefinitions); err != nil {
		return nil, err
	}

	step, err := resolveStep("", start, end, datasource.scrapeInterval)
	if err != nil {
		return nil, err
	}
	promAPI, err := p.newAPI(datasource)
	if err != nil {
		return nil, err
	}
	summaries, queryWarnings := p.runPanelQueries(ctx, promAPI, datasource.ref, input.Panels, promv1.Range{Start: start, End: end, Step: step}, location)

	output := &RenderPanelsOutput{
		Status:     panelsStatusOK,
		Datasource: datasource.ref,
		TimeRange:  timeRange,
		Step:       model.Duration(step).String(),
		Panels:     panelDefinitions,
		Summaries:  summaries,
		Warnings:   dedupe(append(append(warnings, datasource.warnings()...), queryWarnings...)),
	}
	if problems := describeProblems(input, summaries); len(problems) > 0 {
		output.Status = panelsStatusError
		output.Message = "Nothing was displayed. " + strings.Join(problems, " ") +
			" Fix the queries (check the metric and label names) and call the tool again."
		output.Panels = []*dashboard.Panel{}
	}
	return output, nil
}

// checkPanels checks the panels and returns their kinds.
func checkPanels(panels []PanelInput, installed map[string]bool) ([]string, error) {
	if len(panels) == 0 || len(panels) > maxPanels {
		return nil, fmt.Errorf("pass 1 to %d panels", maxPanels)
	}
	kinds := make([]string, len(panels))
	for i, panel := range panels {
		kind, err := checkPanelKind(panel.Kind, installed)
		if err == nil {
			err = checkUnit(panel.Unit, kind)
		}
		if err == nil && (len(panel.Queries) == 0 || len(panel.Queries) > maxPanelQueries) {
			err = fmt.Errorf("pass 1 to %d queries", maxPanelQueries)
		}
		if err != nil {
			return nil, fmt.Errorf("panels[%d] (%q): %w", i, panel.Title, err)
		}
		kinds[i] = kind
	}
	return kinds, nil
}

// validatePanels validates the panels against the plugin schemas of Perses, in a dashboard that is not saved.
func (p *panels) validatePanels(panelDefinitions []*dashboard.Panel) error {
	panelMap := make(map[string]any, len(panelDefinitions))
	for i, panel := range panelDefinitions {
		panelMap[fmt.Sprintf("%s%d", validationPanelKey, i)] = panel
	}
	data, err := json.Marshal(map[string]any{
		"kind":     v1.KindDashboard,
		"metadata": map[string]any{"name": "perses_render_panels"},
		"spec":     map[string]any{"duration": "1h", "panels": panelMap},
	})
	if err != nil {
		return fmt.Errorf("error encoding the panels: %w", err)
	}
	board := &v1.Dashboard{}
	if err = json.Unmarshal(data, board); err != nil {
		return invalidPanelsError(err.Error(), len(panelDefinitions))
	}
	err = p.validate(board)
	if err == nil {
		return nil
	}
	var requestErr *perseshttp.RequestError
	if errors.As(err, &requestErr) && requestErr.StatusCode == http.StatusBadRequest && requestErr.Message != "" {
		return invalidPanelsError(requestErr.Message, len(panelDefinitions))
	}
	return fmt.Errorf("error validating the panels with Perses: %w", err)
}

// invalidPanelsError returns the validation error of Perses, with the panels named by their index in the input.
func invalidPanelsError(message string, count int) error {
	replacements := make([]string, 0, 2*count)
	// The highest indexes first: mcp_panel_1 must not replace the beginning of mcp_panel_10.
	for i := count - 1; i >= 0; i-- {
		replacements = append(replacements, fmt.Sprintf("%s%d", validationPanelKey, i), fmt.Sprintf("panels[%d]", i))
	}
	message = strings.NewReplacer(replacements...).Replace(message)
	// The CUE errors of Perses list the schema files: the beginning is enough to fix the spec.
	if len(message) > maxValidationErrorLength {
		message = message[:maxValidationErrorLength] + "..."
	}
	return fmt.Errorf("invalid panels (fix spec, unit or kind): %s", message)
}

// runPanelQueries runs the queries of the panels as range queries and returns their summaries, in the order of the
// input, and the warnings of Prometheus.
func (p *panels) runPanelQueries(ctx context.Context, promAPI promv1.API, ref DatasourceRef, panels []PanelInput, queryRange promv1.Range, location *time.Location) ([]querySummary, []string) {
	var summaries []querySummary
	for i, panel := range panels {
		for _, query := range panel.Queries {
			summaries = append(summaries, querySummary{PanelIndex: i, Panel: panel.Title, PromQL: query.PromQL, Series: []seriesSummary{}})
		}
	}
	warnings := make([][]string, len(summaries))
	group := errgroup.Group{}
	group.SetLimit(maxParallelQueries)
	queryIndex := 0
	for _, panel := range panels {
		for _, query := range panel.Queries {
			i := queryIndex
			queryIndex++
			group.Go(func() error {
				warnings[i] = summarizeQuery(ctx, promAPI, ref, query, queryRange, location, &summaries[i])
				return nil
			})
		}
	}
	_ = group.Wait()
	return summaries, slices.Concat(warnings...)
}

// summarizeQuery runs a panel query and fills its summary. It returns the warnings of Prometheus.
func summarizeQuery(ctx context.Context, promAPI promv1.API, ref DatasourceRef, query PanelQueryInput, queryRange promv1.Range, location *time.Location, summary *querySummary) []string {
	value, warnings, _, err := promAPI.QueryRange(ctx, query.PromQL, queryRange)
	if err != nil {
		message := apiError(ref, err).Error()
		if len(message) > maxErrorLength {
			message = message[:maxErrorLength] + "..."
		}
		summary.Error = message
		return nil
	}
	matrix, isMatrix := value.(model.Matrix)
	if !isMatrix {
		summary.Error = fmt.Sprintf("unexpected result type '%s'", value.Type())
		return nil
	}
	summary.SeriesCount = len(matrix)
	type ranked struct {
		summary seriesSummary
		max     float64
	}
	var all []ranked
	for _, stream := range matrix {
		if len(stream.Histograms) > 0 {
			summary.HistogramSeries++
			continue
		}
		streamStats, maxValue, ok := seriesStats(stream.Values, location)
		if !ok {
			continue
		}
		all = append(all, ranked{summary: seriesSummary{Name: seriesName(stream.Metric, query.Legend), stats: streamStats}, max: maxValue})
	}
	slices.SortStableFunc(all, func(a, b ranked) int { return cmp.Compare(b.max, a.max) })
	for _, r := range all[:min(len(all), maxSummarySeries)] {
		summary.Series = append(summary.Series, r.summary)
	}
	return warnings
}

// seriesName returns the name of a series: its legend with the label placeholders replaced, else its labels.
func seriesName(metric model.Metric, legend string) string {
	if legend != "" {
		return legendPlaceholder.ReplaceAllStringFunc(legend, func(placeholder string) string {
			return string(metric[model.LabelName(legendPlaceholder.FindStringSubmatch(placeholder)[1])])
		})
	}
	name := string(metric[model.MetricNameLabel])
	labelPairs := make([]string, 0, len(metric))
	for key, value := range metric {
		if key != model.MetricNameLabel {
			labelPairs = append(labelPairs, fmt.Sprintf("%s=%q", key, value))
		}
	}
	if len(labelPairs) == 0 {
		if name == "" {
			return "{}"
		}
		return name
	}
	sort.Strings(labelPairs)
	return fmt.Sprintf("%s{%s}", name, strings.Join(labelPairs, ", "))
}

// describeProblems returns why the panels can't be displayed: failed queries, panels without data unless allowed, and
// histogram panels without native histograms.
func describeProblems(input RenderPanelsInput, summaries []querySummary) []string {
	var problems []string
	for _, summary := range summaries {
		if summary.Error != "" {
			problems = append(problems, fmt.Sprintf("Query `%s` of panels[%d] failed: %s.", summary.PromQL, summary.PanelIndex, summary.Error))
		}
	}
	for i, panel := range input.Panels {
		succeeded, withData, histograms := 0, 0, 0
		for _, summary := range summaries {
			if summary.PanelIndex != i || summary.Error != "" {
				continue
			}
			succeeded++
			if summary.SeriesCount > 0 {
				withData++
			}
			histograms += summary.HistogramSeries
		}
		if succeeded > 0 && withData == 0 {
			if !input.AllowNoData {
				problems = append(problems, fmt.Sprintf("panels[%d] (%q) has no data: its queries returned no series in the "+
					"time range. If no data is the answer, say it, or set allow_no_data to show empty panels.", i, panel.Title))
			}
			continue
		}
		if succeeded > 0 && nativeHistogramKinds[panel.Kind] && histograms == 0 {
			problems = append(problems, fmt.Sprintf("panels[%d] (%q) is a %s, which displays native histograms only: its "+
				"queries returned none (classic _bucket series are not supported). Use another kind, e.g. "+
				"histogram_quantile() in a TimeSeriesChart.", i, panel.Title, panel.Kind))
		}
	}
	return problems
}

func dedupe(values []string) []string {
	var result []string
	for _, value := range values {
		if !slices.Contains(result, value) {
			result = append(result, value)
		}
	}
	return result
}
