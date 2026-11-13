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
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"

	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"golang.org/x/sync/errgroup"
)

const (
	defaultMetricsLimit = 200
	// defaultMetadataMetricsLimit is the default limit of metric names listed with their metadata, which are larger.
	defaultMetadataMetricsLimit = 50
	// maxMetadataMetricsLimit caps the metric names listed with their metadata, which may be looked up one by one.
	maxMetadataMetricsLimit = 200
	defaultLabelsLimit      = 200
	defaultValuesLimit      = 100
	maxListLimit            = 1000
	maxMetadataMetrics      = 20
	// metadataConcurrency is the number of metadata requests sent at the same time.
	metadataConcurrency = 8
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
//
// When the list is too large and search is set without match, it is requested again with search as a regular
// expression matcher on the label, so that Prometheus filters it: the filter isn't sent from the start as it may cost
// Prometheus more than a plain listing.
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
	fetch := func(match Selectors) ([]string, v1.Warnings, v1.Infos, error) {
		var values []string
		if label == "" {
			names, warnings, infos, fetchErr := promAPI.LabelNames(ctx, match, start, end)
			for _, name := range names {
				values = append(values, string(name))
			}
			return values, warnings, infos, fetchErr
		}
		labelValues, warnings, infos, fetchErr := promAPI.LabelValues(ctx, escapeLabelName(label), match, start, end)
		for _, value := range labelValues {
			values = append(values, string(value))
		}
		return values, warnings, infos, fetchErr
	}
	values, warnings, infos, err := fetch(match)
	if errors.Is(err, errTooMuchData) && search != "" && label != "" && len(match) == 0 {
		values, warnings, infos, err = fetch(Selectors{searchSelector(label, search)})
	}
	if errors.Is(err, errTooMuchData) {
		return nil, errListTooLarge
	}
	if err != nil {
		return nil, apiError(datasource.ref, err)
	}
	if search != "" {
		// The same matching as searchSelector, so that a list filtered by Prometheus has the same values.
		matcher := regexp.MustCompile(searchPattern(search))
		values = slices.DeleteFunc(values, func(value string) bool { return !matcher.MatchString(value) })
	}
	if values == nil {
		values = []string{}
	}
	allWarnings := append(datasource.warnings(), warnings...)
	return &listing{datasource: datasource, values: values, warnings: append(allWarnings, infos...)}, nil
}

var errListTooLarge = errors.New("the list is too large (over 1 MiB): narrow it with match (e.g. {job=\"node\"}), " +
	"search or a shorter time_range")

// escapeLabelName returns the name of a label in the path of the label values endpoint, where the Prometheus client
// puts it as is. A UTF-8 name (e.g. http.method) is escaped as Prometheus documents it (U__http_2e_method), which is
// required for '/' and matches the allowedEndpoints patterns of the Perses proxy; other names are kept unchanged.
// Prometheus unescapes every name starting with U__, so such a name is always escaped to be read back unchanged.
func escapeLabelName(label string) string {
	if strings.HasPrefix(label, "U__") && model.LegacyValidation.IsValidMetricName(label) {
		// EscapeName keeps the legacy names unchanged: in them, only the underscores are escaped (doubled).
		return "U__" + strings.ReplaceAll(label, "_", "__")
	}
	return model.EscapeName(label, model.ValueEncodingEscaping)
}

// searchPattern returns the regular expression of the values that contain search, ignoring case. Prometheus uses the
// regular expressions of Go (RE2) for its matchers, so the values match the same way here and in Prometheus.
func searchPattern(search string) string {
	return "(?i)" + regexp.QuoteMeta(search)
}

// searchSelector returns a series selector of the values of label that contain search, ignoring case. Prometheus
// anchors the regular expressions of matchers, hence the surrounding ".*", which also matches newlines with the
// flag s (the default since Prometheus 3 only). A UTF-8 label name is quoted, as Prometheus 3 requires; other names are
// not, so that the selector also works with older versions.
func searchSelector(label, search string) string {
	name := label
	if !model.LegacyValidation.IsValidLabelName(label) {
		name = strconv.Quote(label)
	}
	return fmt.Sprintf("{%s=~%s}", name, strconv.Quote("(?is).*"+regexp.QuoteMeta(search)+".*"))
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
	limit, defaultLimit := input.Limit, defaultMetricsLimit
	if input.IncludeMetadata {
		defaultLimit = defaultMetadataMetricsLimit
		if limit > maxMetadataMetricsLimit {
			limit = maxMetadataMetricsLimit
		}
	}
	returned, namesPage := firstPage(names.values, limit, defaultLimit)
	output := &metricNamesOutput{Datasource: names.datasource.ref, page: namesPage, Metrics: returned, Warnings: names.warnings}
	if input.IncludeMetadata {
		var metadata map[string][]v1.Metadata
		var metadataErr error
		if len(returned) > 0 {
			metadata, metadataErr = p.metadata(ctx, names.datasource, "")
			if errors.Is(metadataErr, errTooMuchData) {
				// The metadata of all the metrics is too large: only the returned metrics are looked up.
				metadata, metadataErr = p.lookupMetadata(ctx, names.datasource, returned)
			}
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
// The metadata of these series may be under the family name, e.g. http_requests for http_requests_total. They are the
// suffixes with which Prometheus matches metadata, plus _created (OpenMetrics), which gauge histograms don't have.
var familySuffixes = []struct {
	suffix string
	types  []v1.MetricType
}{
	{"_bucket", []v1.MetricType{v1.MetricTypeHistogram, v1.MetricTypeGaugeHistogram}},
	{"_count", []v1.MetricType{v1.MetricTypeHistogram, v1.MetricTypeGaugeHistogram, v1.MetricTypeSummary}},
	{"_sum", []v1.MetricType{v1.MetricTypeHistogram, v1.MetricTypeGaugeHistogram, v1.MetricTypeSummary}},
	{"_gcount", []v1.MetricType{v1.MetricTypeGaugeHistogram}},
	{"_gsum", []v1.MetricType{v1.MetricTypeGaugeHistogram}},
	{"_created", []v1.MetricType{v1.MetricTypeCounter, v1.MetricTypeHistogram, v1.MetricTypeSummary}},
	{"_total", []v1.MetricType{v1.MetricTypeCounter}},
	{"_info", []v1.MetricType{v1.MetricTypeInfo}},
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

// lookupMetadata looks up the metadata of each metric (metricMetadata), metadataConcurrency at a time, and returns
// the metadata found by metric name. The first error cancels the lookups in progress and is returned.
func (p *prometheus) lookupMetadata(ctx context.Context, datasource resolvedDatasource, metrics []string) (map[string][]v1.Metadata, error) {
	results := make([][]v1.Metadata, len(metrics))
	group, ctx := errgroup.WithContext(ctx)
	group.SetLimit(metadataConcurrency)
	for i, metric := range metrics {
		group.Go(func() error {
			var err error
			results[i], err = p.metricMetadata(ctx, datasource, metric)
			return err
		})
	}
	// The lookups write results from their goroutines: it can only be read once all of them have returned.
	if err := group.Wait(); err != nil {
		return nil, err
	}
	metadata := make(map[string][]v1.Metadata, len(metrics))
	for i, metric := range metrics {
		if len(results[i]) > 0 {
			metadata[metric] = results[i]
		}
	}
	return metadata, nil
}

func (p *prometheus) getMetricMetadata(ctx context.Context, input GetMetricMetadataInput) (*metricMetadataOutput, error) {
	if len(input.Metrics) > maxMetadataMetrics {
		return nil, fmt.Errorf("too many metrics: at most %d per call", maxMetadataMetrics)
	}
	datasource, err := p.resolver.resolve(input.Datasource)
	if err != nil {
		return nil, err
	}
	metadata, err := p.lookupMetadata(ctx, datasource, input.Metrics)
	if err != nil {
		return nil, err
	}
	output := &metricMetadataOutput{Datasource: datasource.ref, Metadata: metadata, Missing: []string{}, Warnings: datasource.warnings()}
	for _, metric := range input.Metrics {
		if _, found := metadata[metric]; !found {
			output.Missing = append(output.Missing, metric)
		}
	}
	return output, nil
}
