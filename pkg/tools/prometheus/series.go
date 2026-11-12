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
	"errors"
	"math"
	"slices"
	"strconv"
	"time"

	"github.com/prometheus/common/model"
)

// timeLayout formats minAt and maxAt, e.g. "2 Jan 2026, 14:05".
const timeLayout = "2 Jan 2006, 15:04"

var errNativeHistograms = errors.New("the query returns native histograms, which this tool does not summarize: " +
	"use histogram_quantile(), histogram_count() or histogram_sum() to get numbers")

// stats are the statistics of a series of a range query. Min, max and avg leave out NaN and infinite samples.
type stats struct {
	Min float64 `json:"min"`
	Max float64 `json:"max"`
	Avg float64 `json:"avg"`
	// Last is the last sample: a number, or a string for NaN and infinities.
	Last  any    `json:"last"`
	MinAt string `json:"minAt"`
	MaxAt string `json:"maxAt"`
	// NonFinite is the number of NaN and infinite samples.
	NonFinite int `json:"nonFinite,omitempty"`
}

// series is a series of a query result: its value for an instant query, its statistics for a range query.
type series struct {
	Labels map[string]string `json:"labels"`
	// Value is a number, or a string for NaN and infinities.
	Value any `json:"value,omitempty"`
	*stats
	// sortKey orders the series, highest first.
	sortKey float64
}

func vectorSeries(vector model.Vector) ([]series, error) {
	result := make([]series, 0, len(vector))
	for _, sample := range vector {
		if sample.Histogram != nil {
			return nil, errNativeHistograms
		}
		value, sortKey := sampleValue(float64(sample.Value))
		result = append(result, series{Labels: labels(sample.Metric), Value: value, sortKey: sortKey})
	}
	return result, nil
}

// matrixSeries returns the statistics of the series of a range query, and the number of series left out because
// they have no finite sample.
func matrixSeries(matrix model.Matrix, location *time.Location) ([]series, int, error) {
	result := make([]series, 0, len(matrix))
	withoutFiniteSample := 0
	for _, stream := range matrix {
		if len(stream.Histograms) > 0 {
			return nil, 0, errNativeHistograms
		}
		streamStats, maxValue, ok := seriesStats(stream.Values, location)
		if !ok {
			withoutFiniteSample++
			continue
		}
		result = append(result, series{Labels: labels(stream.Metric), stats: &streamStats, sortKey: maxValue})
	}
	return result, withoutFiniteSample, nil
}

// keepTopSeries sorts the series by decreasing sort key, keeps the first limit ones and moves the labels shared by
// all of them to the returned common labels.
func keepTopSeries(all []series, limit int) ([]series, map[string]string) {
	slices.SortStableFunc(all, func(a, b series) int { return cmp.Compare(b.sortKey, a.sortKey) })
	top := all[:min(limit, len(all))]
	if len(top) < 2 {
		return top, nil
	}
	common := map[string]string{}
	for key, value := range top[0].Labels {
		shared := true
		for _, other := range top[1:] {
			if otherValue, ok := other.Labels[key]; !ok || otherValue != value {
				shared = false
				break
			}
		}
		if shared {
			common[key] = value
		}
	}
	if len(common) == 0 {
		return top, nil
	}
	for _, s := range top {
		for key := range common {
			delete(s.Labels, key)
		}
	}
	return top, common
}

func labels(metric model.Metric) map[string]string {
	result := make(map[string]string, len(metric))
	for name, value := range metric {
		result[string(name)] = string(value)
	}
	return result
}

// sampleValue returns the rounded value of a sample, or a string for NaN and infinities, and its sort key (NaN last).
func sampleValue(value float64) (any, float64) {
	switch {
	case math.IsNaN(value):
		return "NaN", math.Inf(-1)
	case math.IsInf(value, 1):
		return "+Inf", value
	case math.IsInf(value, -1):
		return "-Inf", value
	}
	return round(value), value
}

// seriesStats computes the statistics of the samples of a series and returns its unrounded maximum. ok is false when
// the series has no finite sample.
func seriesStats(samples []model.SamplePair, location *time.Location) (result stats, maxValue float64, ok bool) {
	var (
		minValue, avg float64
		minAt, maxAt  model.Time
		count         int
	)
	for _, sample := range samples {
		value := float64(sample.Value)
		if math.IsNaN(value) || math.IsInf(value, 0) {
			result.NonFinite++
			continue
		}
		count++
		if count == 1 || value < minValue {
			minValue, minAt = value, sample.Timestamp
		}
		if count == 1 || value > maxValue {
			maxValue, maxAt = value, sample.Timestamp
		}
		// A running mean, weighted so that no intermediate value exceeds the samples: a sum of large values could overflow.
		avg = avg*(float64(count-1)/float64(count)) + value/float64(count)
	}
	if count == 0 {
		return stats{}, 0, false
	}
	result.Min = round(minValue)
	result.Max = round(maxValue)
	result.Avg = round(avg)
	result.Last, _ = sampleValue(float64(samples[len(samples)-1].Value))
	result.MinAt = minAt.Time().In(location).Format(timeLayout)
	result.MaxAt = maxAt.Time().In(location).Format(timeLayout)
	return result, maxValue, true
}

// round keeps 4 significant digits.
func round(value float64) float64 {
	rounded, err := strconv.ParseFloat(strconv.FormatFloat(value, 'g', 4, 64), 64)
	if err != nil {
		return value
	}
	return rounded
}
