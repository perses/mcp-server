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
	"errors"
	"fmt"
	"time"

	"github.com/prometheus/common/model"
)

const (
	// defaultPastDuration is the time range used when none is given.
	defaultPastDuration = "1h"
	// statsPoints is the number of points requested by default for range queries.
	statsPoints = 500
	// maxPoints is the Prometheus limit of points per series of a range query.
	maxPoints = 11000
)

// TimeRangeInput is a time range of the tool inputs: relative (PastDuration) or absolute (Start and End).
type TimeRangeInput struct {
	PastDuration string `json:"past_duration,omitempty"`
	Start        string `json:"start,omitempty"`
	End          string `json:"end,omitempty"`
}

// timeRangeOutput is the resolved time range returned by the tools: relative (PastDuration) or absolute (Start and End,
// RFC 3339 in UTC).
type timeRangeOutput struct {
	PastDuration string `json:"pastDuration,omitempty"`
	Start        string `json:"start,omitempty"`
	End          string `json:"end,omitempty"`
}

// resolveTimeRange returns the start and end of the time range and its representation in the output: the absolute
// range when start is set (end defaults to now), otherwise the past duration (default 1h) ending now.
func resolveTimeRange(input *TimeRangeInput, now time.Time) (time.Time, time.Time, timeRangeOutput, error) {
	if input == nil {
		input = &TimeRangeInput{}
	}
	if input.Start == "" {
		if input.End != "" {
			return time.Time{}, time.Time{}, timeRangeOutput{}, errors.New("time_range.end requires time_range.start")
		}
		pastDuration := input.PastDuration
		if pastDuration == "" {
			pastDuration = defaultPastDuration
		}
		duration, err := parseDuration(pastDuration)
		if err != nil {
			return time.Time{}, time.Time{}, timeRangeOutput{}, fmt.Errorf("invalid time_range.past_duration '%s': %w", pastDuration, err)
		}
		return now.Add(-duration), now, timeRangeOutput{PastDuration: pastDuration}, nil
	}
	if input.PastDuration != "" {
		return time.Time{}, time.Time{}, timeRangeOutput{}, errors.New("set either time_range.past_duration or time_range.start, not both")
	}
	start, err := parseTime("time_range.start", input.Start)
	if err != nil {
		return time.Time{}, time.Time{}, timeRangeOutput{}, err
	}
	end := now
	if input.End != "" {
		if end, err = parseTime("time_range.end", input.End); err != nil {
			return time.Time{}, time.Time{}, timeRangeOutput{}, err
		}
	}
	if !start.Before(end) {
		return time.Time{}, time.Time{}, timeRangeOutput{}, errors.New("time_range.start must be before time_range.end")
	}
	return start, end, timeRangeOutput{Start: start.UTC().Format(time.RFC3339), End: end.UTC().Format(time.RFC3339)}, nil
}

// parseDuration parses a positive Prometheus duration, e.g. "15m", "1h30m" or "7d".
func parseDuration(duration string) (time.Duration, error) {
	parsed, err := model.ParseDuration(duration)
	if err != nil {
		return 0, fmt.Errorf("use a duration like 15m, 1h or 7d: %w", err)
	}
	if parsed <= 0 {
		return 0, errors.New("the duration must be positive")
	}
	return time.Duration(parsed), nil
}

func parseTime(field, value string) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid %s '%s': use RFC 3339, e.g. 2026-01-02T15:04:05Z", field, value)
	}
	return parsed, nil
}

// resolveStep returns the step of a range query: the given step, raised if needed to stay under the Prometheus
// limit of points, or by default about statsPoints points and at least minStep, the scrape interval of the datasource
// (a smaller step only repeats the samples).
func resolveStep(step string, start, end time.Time, minStep time.Duration) (time.Duration, error) {
	span := end.Sub(start)
	if step == "" {
		return max(minStep, ceilTo(span/statsPoints, time.Second)), nil
	}
	parsed, err := parseStep(step)
	if err != nil {
		return 0, err
	}
	return max(parsed, minLimitStep(span)), nil
}

// minLimitStep returns the smallest step that stays under the Prometheus limit of points over span, rounded up to the
// second, or to the millisecond (the precision of the steps) when it is under a second: a valid subsecond step is
// kept.
func minLimitStep(span time.Duration) time.Duration {
	limit := (span + maxPoints - 1) / maxPoints
	if limit < time.Second {
		return ceilTo(limit, time.Millisecond)
	}
	return ceilTo(limit, time.Second)
}

func parseStep(step string) (time.Duration, error) {
	parsed, err := parseDuration(step)
	if err != nil {
		return 0, fmt.Errorf("invalid step '%s': %w", step, err)
	}
	return parsed, nil
}

// ceilTo rounds a duration up to a multiple of unit.
func ceilTo(duration, unit time.Duration) time.Duration {
	return (duration + unit - 1).Truncate(unit)
}

// resolveLocation returns the IANA time zone used to format times, UTC by default.
func resolveLocation(timeZone string) (*time.Location, error) {
	if timeZone == "" {
		return time.UTC, nil
	}
	location, err := time.LoadLocation(timeZone)
	if err != nil {
		return nil, fmt.Errorf("invalid time_zone '%s': use an IANA time zone, e.g. Europe/Berlin", timeZone)
	}
	return location, nil
}
