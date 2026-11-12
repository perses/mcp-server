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
	"errors"
	"fmt"
	"time"

	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
)

const (
	instantQuery       = "instant"
	rangeQuery         = "range"
	defaultSeriesLimit = 20
	maxSeriesLimit     = 100
)

type QueryInput struct {
	Datasource *DatasourceRef  `json:"datasource,omitempty"`
	Query      string          `json:"query"`
	Type       string          `json:"type,omitempty"`
	Time       string          `json:"time,omitempty"`
	TimeRange  *TimeRangeInput `json:"time_range,omitempty"`
	Step       string          `json:"step,omitempty"`
	Limit      int             `json:"limit,omitempty"`
	TimeZone   string          `json:"time_zone,omitempty"`
}

type queryOutput struct {
	Datasource   DatasourceRef     `json:"datasource"`
	ResultType   string            `json:"resultType"`
	Time         string            `json:"time,omitempty"`
	TimeRange    *timeRangeOutput  `json:"timeRange,omitempty"`
	Step         string            `json:"step,omitempty"`
	SeriesCount  *int              `json:"seriesCount,omitempty"`
	Returned     *int              `json:"returned,omitempty"`
	CommonLabels map[string]string `json:"commonLabels,omitempty"`
	Series       []series          `json:"series,omitempty"`
	// Value is the value of a scalar or string result.
	Value    any      `json:"value,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
	Infos    []string `json:"infos,omitempty"`
}

func (p *prometheus) query(ctx context.Context, input QueryInput) (*queryOutput, error) {
	location, err := resolveLocation(input.TimeZone)
	if err != nil {
		return nil, err
	}
	limit := input.Limit
	if limit <= 0 {
		limit = defaultSeriesLimit
	}
	limit = min(limit, maxSeriesLimit)

	// The input is checked before any request.
	output := &queryOutput{}
	var at time.Time
	var queryRange v1.Range
	switch input.Type {
	case rangeQuery:
		if input.Time != "" {
			return nil, errors.New("time applies to instant queries: use time_range for a range query")
		}
		start, end, timeRange, rangeErr := resolveTimeRange(input.TimeRange, p.now())
		if rangeErr != nil {
			return nil, rangeErr
		}
		step, stepErr := resolveStep(input.Step, start, end)
		if stepErr != nil {
			return nil, stepErr
		}
		queryRange = v1.Range{Start: start, End: end, Step: step}
		output.TimeRange = &timeRange
		output.Step = model.Duration(step).String()
	case instantQuery, "":
		if input.TimeRange != nil || input.Step != "" {
			return nil, errors.New("time_range and step apply to range queries: set type to 'range', or use time for an instant query")
		}
		at = p.now()
		if input.Time != "" {
			if at, err = parseTime("time", input.Time); err != nil {
				return nil, err
			}
		}
		output.Time = at.UTC().Format(time.RFC3339)
	default:
		return nil, fmt.Errorf("invalid type '%s': use '%s' or '%s'", input.Type, instantQuery, rangeQuery)
	}

	datasource, err := p.resolver.resolve(input.Datasource)
	if err != nil {
		return nil, err
	}
	output.Datasource = datasource
	promAPI, err := p.newAPI(datasource)
	if err != nil {
		return nil, err
	}
	var value model.Value
	var warnings v1.Warnings
	var infos v1.Infos
	if input.Type == rangeQuery {
		value, warnings, infos, err = promAPI.QueryRange(ctx, input.Query, queryRange)
	} else {
		value, warnings, infos, err = promAPI.Query(ctx, input.Query, at)
	}
	if err != nil {
		return nil, apiError(datasource, err)
	}
	output.Warnings = append([]string(nil), warnings...)
	output.Infos = infos

	var all []series
	switch result := value.(type) {
	case model.Vector:
		output.ResultType = "vector"
		output.SeriesCount = new(len(result))
		if all, err = vectorSeries(result); err != nil {
			return nil, err
		}
	case model.Matrix:
		output.ResultType = "matrix"
		output.SeriesCount = new(len(result))
		var withoutFiniteSample int
		if all, withoutFiniteSample, err = matrixSeries(result, location); err != nil {
			return nil, err
		}
		if withoutFiniteSample > 0 {
			output.Warnings = append(output.Warnings, fmt.Sprintf("%d series are not returned: they only have NaN or infinite samples in the time range", withoutFiniteSample))
		}
	case *model.Scalar:
		output.ResultType = "scalar"
		output.Value, _ = sampleValue(float64(result.Value))
		return output, nil
	case *model.String:
		output.ResultType = "string"
		output.Value = result.Value
		return output, nil
	default:
		return nil, fmt.Errorf("unexpected result type '%s' from %s", value.Type(), datasource)
	}
	output.Series, output.CommonLabels = keepTopSeries(all, limit)
	output.Returned = new(len(output.Series))
	return output, nil
}
