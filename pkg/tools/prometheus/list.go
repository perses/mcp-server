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
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"sync"

	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
)

const (
	defaultMetricsLimit = 200
	// defaultMetadataMetricsLimit is the default limit of metric names listed with their metadata, which are larger.
	defaultMetadataMetricsLimit = 50
	defaultLabelsLimit          = 200
	defaultValuesLimit          = 100
	maxListLimit                = 1000
	maxMetadataMetrics          = 20
	// maxHelpLength caps the help of metrics listed with their metadata.
	maxHelpLength = 80
)

// Selectors are series selectors (match[]), given as a list or as a single selector.
type Selectors []string

func (s *Selectors) UnmarshalJSON(data []byte) error {
	var single string
	if err := json.Unmarshal(data, &single); err == nil {
		*s = nil
		if single != "" {
			*s = Selectors{single}
		}
		return nil
	}
	var list []string
	if err := json.Unmarshal(data, &list); err != nil {
		return err
	}
	*s = list
	return nil
}

type ListMetricNamesInput struct {
	Datasource      *DatasourceRef  `json:"datasource,omitempty"`
	Match           Selectors       `json:"match,omitempty"`
	Search          string          `json:"search,omitempty"`
	TimeRange       *TimeRangeInput `json:"time_range,omitempty"`
	Limit           int             `json:"limit,omitempty"`
	IncludeMetadata bool            `json:"include_metadata,omitempty"`
}

type ListLabelNamesInput struct {
	Datasource *DatasourceRef  `json:"datasource,omitempty"`
	Match      Selectors       `json:"match,omitempty"`
	TimeRange  *TimeRangeInput `json:"time_range,omitempty"`
	Limit      int             `json:"limit,omitempty"`
}

type ListLabelValuesInput struct {
	Datasource *DatasourceRef  `json:"datasource,omitempty"`
	Label      string          `json:"label"`
	Match      Selectors       `json:"match,omitempty"`
	Search     string          `json:"search,omitempty"`
	TimeRange  *TimeRangeInput `json:"time_range,omitempty"`
	Limit      int             `json:"limit,omitempty"`
}

type GetMetricMetadataInput struct {
	Datasource *DatasourceRef `json:"datasource,omitempty"`
	Metrics    []string       `json:"metrics"`
}

// page tells which part of a list is returned.
type page struct {
	Total     int  `json:"total"`
	Returned  int  `json:"returned"`
	Truncated bool `json:"truncated"`
}

type metricInfo struct {
	Name string `json:"name"`
	Type string `json:"type,omitempty"`
	Help string `json:"help,omitempty"`
}

type metricNamesOutput struct {
	Datasource DatasourceRef `json:"datasource"`
	page
	// Metrics holds names, or metricInfo with include_metadata.
	Metrics  any      `json:"metrics"`
	Warnings []string `json:"warnings,omitempty"`
}

type labelNamesOutput struct {
	Datasource DatasourceRef `json:"datasource"`
	page
	Labels   []string `json:"labels"`
	Warnings []string `json:"warnings,omitempty"`
}

type labelValuesOutput struct {
	Datasource DatasourceRef `json:"datasource"`
	Label      string        `json:"label"`
	page
	Values   []string `json:"values"`
	Warnings []string `json:"warnings,omitempty"`
}

type metricMetadataOutput struct {
	Datasource DatasourceRef            `json:"datasource"`
	Metadata   map[string][]v1.Metadata `json:"metadata"`
	Missing    []string                 `json:"missing"`
	Warnings   []string                 `json:"warnings,omitempty"`
}

// listing is the result of a label names or label values request.
type listing struct {
	datasource resolvedDatasource
	values     []string
	warnings   []string
}

// list resolves the time range and the datasource, then lists the label names (label empty) or the values of a
// label, filtered by search (case-insensitive).
func (p *prometheus) list(ctx context.Context, datasourceInput *DatasourceRef, label string, match Selectors, timeRange *TimeRangeInput, search string) (*listing, error) {
	start, end, _, err := resolveTimeRange(timeRange, p.now())
	if err != nil {
		return nil, err
	}
	datasource, err := p.resolver.resolve(datasourceInput)
	if err != nil {
		return nil, err
	}
	promAPI, err := p.newAPI(datasource)
	if err != nil {
		return nil, err
	}
	var values []string
	var warnings v1.Warnings
	var infos v1.Infos
	if label == "" {
		var names model.LabelNames
		names, warnings, infos, err = promAPI.LabelNames(ctx, match, start, end)
		for _, name := range names {
			values = append(values, string(name))
		}
	} else {
		var labelValues model.LabelValues
		labelValues, warnings, infos, err = promAPI.LabelValues(ctx, label, match, start, end)
		for _, value := range labelValues {
			values = append(values, string(value))
		}
	}
	if err != nil {
		return nil, apiError(datasource.ref, err)
	}
	if search != "" {
		needle := strings.ToLower(search)
		values = slices.DeleteFunc(values, func(value string) bool { return !strings.Contains(strings.ToLower(value), needle) })
	}
	if values == nil {
		values = []string{}
	}
	allWarnings := append(datasource.warnings(), warnings...)
	return &listing{datasource: datasource, values: values, warnings: append(allWarnings, infos...)}, nil
}

// firstPage returns the first limit values (defaultLimit when limit is not set) and the page they form.
func firstPage(values []string, limit, defaultLimit int) ([]string, page) {
	if limit <= 0 {
		limit = defaultLimit
	}
	returned := values[:min(limit, maxListLimit, len(values))]
	return returned, page{Total: len(values), Returned: len(returned), Truncated: len(returned) < len(values)}
}

func (p *prometheus) listMetricNames(ctx context.Context, input ListMetricNamesInput) (*metricNamesOutput, error) {
	names, err := p.list(ctx, input.Datasource, model.MetricNameLabel, input.Match, input.TimeRange, input.Search)
	if err != nil {
		return nil, err
	}
	defaultLimit := defaultMetricsLimit
	if input.IncludeMetadata {
		defaultLimit = defaultMetadataMetricsLimit
	}
	returned, namesPage := firstPage(names.values, input.Limit, defaultLimit)
	output := &metricNamesOutput{Datasource: names.datasource.ref, page: namesPage, Metrics: returned, Warnings: names.warnings}
	if input.IncludeMetadata {
		var metadata map[string][]v1.Metadata
		var metadataErr error
		if len(returned) > 0 {
			metadata, metadataErr = p.metadata(ctx, names.datasource, "")
		}
		if metadataErr != nil {
			output.Warnings = append(output.Warnings, fmt.Sprintf("metadata unavailable (%s): use %s for specific metrics",
				metadataErr, GetMetricMetadataToolName))
		}
		infos := make([]metricInfo, 0, len(returned))
		for _, name := range returned {
			info := metricInfo{Name: name}
			if entries := findMetadata(metadata, name); len(entries) > 0 {
				info.Type = string(entries[0].Type)
				info.Help = shortenHelp(entries[0].Help)
			}
			infos = append(infos, info)
		}
		output.Metrics = infos
	}
	return output, nil
}

func (p *prometheus) listLabelNames(ctx context.Context, input ListLabelNamesInput) (*labelNamesOutput, error) {
	labels, err := p.list(ctx, input.Datasource, "", input.Match, input.TimeRange, "")
	if err != nil {
		return nil, err
	}
	returned, labelsPage := firstPage(labels.values, input.Limit, defaultLabelsLimit)
	return &labelNamesOutput{Datasource: labels.datasource.ref, page: labelsPage, Labels: returned, Warnings: labels.warnings}, nil
}

func (p *prometheus) listLabelValues(ctx context.Context, input ListLabelValuesInput) (*labelValuesOutput, error) {
	values, err := p.list(ctx, input.Datasource, input.Label, input.Match, input.TimeRange, input.Search)
	if err != nil {
		return nil, err
	}
	returned, valuesPage := firstPage(values.values, input.Limit, defaultValuesLimit)
	return &labelValuesOutput{Datasource: values.datasource.ref, Label: input.Label, page: valuesPage, Values: returned, Warnings: values.warnings}, nil
}

// familySuffixes are the suffixes of the series of a metric family, with the types of the families that have them.
// The metadata of these series may be under the family name, e.g. http_requests for http_requests_total.
var familySuffixes = []struct {
	suffix string
	types  []v1.MetricType
}{
	{"_bucket", []v1.MetricType{v1.MetricTypeHistogram, v1.MetricTypeGaugeHistogram}},
	{"_count", []v1.MetricType{v1.MetricTypeHistogram, v1.MetricTypeGaugeHistogram, v1.MetricTypeSummary}},
	{"_sum", []v1.MetricType{v1.MetricTypeHistogram, v1.MetricTypeGaugeHistogram, v1.MetricTypeSummary}},
	{"_created", []v1.MetricType{v1.MetricTypeCounter, v1.MetricTypeHistogram, v1.MetricTypeGaugeHistogram, v1.MetricTypeSummary}},
	{"_total", []v1.MetricType{v1.MetricTypeCounter}},
}

// familyName returns the name of the metric family a series name may belong to, and the types of such a family.
func familyName(name string) (string, []v1.MetricType, bool) {
	for _, family := range familySuffixes {
		if base, found := strings.CutSuffix(name, family.suffix); found && base != "" {
			return base, family.types, true
		}
	}
	return "", nil, false
}

// findMetadata returns the metadata of a metric: its own, else the metadata of its family when the type of the
// family has series with this suffix (e.g. a histogram for _bucket, a counter for _total).
func findMetadata(metadata map[string][]v1.Metadata, name string) []v1.Metadata {
	if entries := metadata[name]; len(entries) > 0 {
		return entries
	}
	family, types, ok := familyName(name)
	if !ok {
		return nil
	}
	var entries []v1.Metadata
	for _, entry := range metadata[family] {
		if slices.Contains(types, entry.Type) {
			entries = append(entries, entry)
		}
	}
	return entries
}

func shortenHelp(help string) string {
	runes := []rune(help)
	if len(runes) <= maxHelpLength {
		return help
	}
	return strings.TrimSpace(string(runes[:maxHelpLength-1])) + "…"
}

// metadata returns the metadata of a metric, or of all the metrics when metric is empty.
func (p *prometheus) metadata(ctx context.Context, datasource resolvedDatasource, metric string) (map[string][]v1.Metadata, error) {
	promAPI, err := p.newAPI(datasource)
	if err != nil {
		return nil, err
	}
	metadata, err := promAPI.Metadata(ctx, metric, "")
	if err != nil {
		return nil, apiError(datasource.ref, err)
	}
	return metadata, nil
}

// metricMetadata returns the metadata of a metric, looked up under its own name first, then under its family name.
func (p *prometheus) metricMetadata(ctx context.Context, datasource resolvedDatasource, metric string) ([]v1.Metadata, error) {
	metadata, err := p.metadata(ctx, datasource, metric)
	if err != nil {
		return nil, err
	}
	if entries := metadata[metric]; len(entries) > 0 {
		return entries, nil
	}
	family, _, ok := familyName(metric)
	if !ok {
		return nil, nil
	}
	if metadata, err = p.metadata(ctx, datasource, family); err != nil {
		return nil, err
	}
	return findMetadata(metadata, metric), nil
}

func (p *prometheus) getMetricMetadata(ctx context.Context, input GetMetricMetadataInput) (*metricMetadataOutput, error) {
	if len(input.Metrics) > maxMetadataMetrics {
		return nil, fmt.Errorf("too many metrics: at most %d per call", maxMetadataMetrics)
	}
	datasource, err := p.resolver.resolve(input.Datasource)
	if err != nil {
		return nil, err
	}
	results := make([][]v1.Metadata, len(input.Metrics))
	errs := make([]error, len(input.Metrics))
	var wg sync.WaitGroup
	for i, metric := range input.Metrics {
		wg.Go(func() {
			results[i], errs[i] = p.metricMetadata(ctx, datasource, metric)
		})
	}
	wg.Wait()
	output := &metricMetadataOutput{Datasource: datasource.ref, Metadata: map[string][]v1.Metadata{}, Missing: []string{}, Warnings: datasource.warnings()}
	for i, metric := range input.Metrics {
		if errs[i] != nil {
			return nil, errs[i]
		}
		if len(results[i]) == 0 {
			output.Missing = append(output.Missing, metric)
			continue
		}
		output.Metadata[metric] = results[i]
	}
	return output, nil
}
