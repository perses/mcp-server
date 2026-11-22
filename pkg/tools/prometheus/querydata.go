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
	"math"
	"strconv"
	"time"

	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
)

// maxQueryDataBytes caps the response of perses_query_prometheus_data: the hosts pass it to the view in one message, and
// some of them limit the size of the tool results.
var maxQueryDataBytes = 2 << 20

// QueryDataInput is a query of the Prometheus HTTP API, as sent by the Prometheus plugin of the view: an instant
// query at Time, or a range query when Start is set. Times are Unix timestamps in seconds or RFC 3339, steps
// durations or seconds.
type QueryDataInput struct {
	Datasource *DatasourceRef `json:"datasource,omitempty"`
	Query      string         `json:"query"`
	Time       string         `json:"time,omitempty"`
	Start      string         `json:"start,omitempty"`
	End        string         `json:"end,omitempty"`
	Step       string         `json:"step,omitempty"`
}

// queryData runs a query and returns the response of the Prometheus HTTP API (status, data, warnings, infos).
func (p *panels) queryData(ctx context.Context, input QueryDataInput) (map[string]any, error) {
	var queryRange promv1.Range
	var at time.Time
	var err error
	isRange := input.Start != ""
	if isRange {
		if input.Time != "" {
			return nil, errors.New("set either time (instant query) or start, end and step (range query)")
		}
		if queryRange.Start, err = parseAPITime("start", input.Start); err != nil {
			return nil, err
		}
		if queryRange.End, err = parseAPITime("end", input.End); err != nil {
			return nil, err
		}
		if queryRange.Step, err = parseAPIDuration("step", input.Step); err != nil {
			return nil, err
		}
		if queryRange.End.Before(queryRange.Start) {
			return nil, errors.New("end must not be before start")
		}
	} else {
		if input.End != "" || input.Step != "" {
			return nil, errors.New("end and step require start (range query)")
		}
		at = p.now()
		if input.Time != "" {
			if at, err = parseAPITime("time", input.Time); err != nil {
				return nil, err
			}
		}
	}

	datasource, err := p.resolver.resolve(input.Datasource)
	if err != nil {
		return nil, err
	}
	promAPI, err := p.newAPI(datasource)
	if err != nil {
		return nil, err
	}
	var value model.Value
	var warnings promv1.Warnings
	var infos promv1.Infos
	if isRange {
		value, warnings, infos, err = promAPI.QueryRange(ctx, input.Query, queryRange)
	} else {
		value, warnings, infos, err = promAPI.Query(ctx, input.Query, at)
	}
	if err != nil {
		return nil, apiError(datasource.ref, err)
	}
	response := map[string]any{
		"status": "success",
		"data":   map[string]any{"resultType": value.Type().String(), "result": value},
	}
	if len(warnings) > 0 {
		response["warnings"] = warnings
	}
	if len(infos) > 0 {
		response["infos"] = infos
	}
	if err = checkQueryDataSize(response, value); err != nil {
		return nil, err
	}
	return response, nil
}

// checkQueryDataSize returns an error when the response is larger than maxQueryDataBytes.
func checkQueryDataSize(response map[string]any, value model.Value) error {
	data, err := json.Marshal(response)
	if err != nil {
		return fmt.Errorf("error encoding the response of Prometheus: %w", err)
	}
	if len(data) <= maxQueryDataBytes {
		return nil
	}
	series := ""
	switch v := value.(type) {
	case model.Matrix:
		series = fmt.Sprintf("%d series, ", len(v))
	case model.Vector:
		series = fmt.Sprintf("%d series, ", len(v))
	}
	return fmt.Errorf("the result is too large to display (%s%.1f MiB, the limit is %.0f MiB): aggregate the query "+
		"(e.g. sum by, topk) or shorten the time range", series, float64(len(data))/(1<<20), float64(maxQueryDataBytes)/(1<<20))
}

// parseAPITime parses a time of the Prometheus HTTP API: a Unix timestamp in seconds or RFC 3339.
func parseAPITime(field, value string) (time.Time, error) {
	if value == "" {
		return time.Time{}, fmt.Errorf("%s is required", field)
	}
	if seconds, err := strconv.ParseFloat(value, 64); err == nil && !math.IsNaN(seconds) && !math.IsInf(seconds, 0) {
		whole, fraction := math.Modf(seconds)
		return time.Unix(int64(whole), int64(math.Round(fraction*1e3))*int64(time.Millisecond)).UTC(), nil
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid %s '%s': use a Unix timestamp in seconds or RFC 3339", field, value)
	}
	return parsed, nil
}

// parseAPIDuration parses a positive duration of the Prometheus HTTP API: seconds (e.g. "15" or "0.5") or a
// Prometheus duration (e.g. "15s").
func parseAPIDuration(field, value string) (time.Duration, error) {
	if value == "" {
		return 0, fmt.Errorf("%s is required", field)
	}
	if seconds, err := strconv.ParseFloat(value, 64); err == nil {
		duration := time.Duration(seconds * float64(time.Second))
		if math.IsNaN(seconds) || duration <= 0 {
			return 0, fmt.Errorf("invalid %s '%s': it must be positive", field, value)
		}
		return duration, nil
	}
	duration, err := parseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("invalid %s '%s': %w", field, value, err)
	}
	return duration, nil
}
