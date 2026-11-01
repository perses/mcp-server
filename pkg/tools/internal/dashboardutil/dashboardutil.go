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

// Package dashboardutil contains the dashboard logic shared by the MCP tools: decoding the dashboard argument,
// compact summaries and grid layout placement. Validation is left to the Perses server, whose messages are returned
// unchanged.
package dashboardutil

import (
	"bytes"
	"encoding/json"
	"strconv"
	"strings"

	v1 "github.com/perses/perses/pkg/model/api/v1"
)

// DecodeDashboard decodes the dashboard argument of a tool into the Perses model. kind defaults to Dashboard and
// metadata.project to project. The decoding error is returned unchanged.
func DecodeDashboard(raw json.RawMessage, project string) (*v1.Dashboard, error) {
	document, err := WithDefaultKind(raw, string(v1.KindDashboard))
	if err != nil {
		return nil, err
	}
	var dashboard v1.Dashboard
	if err := json.Unmarshal(document, &dashboard); err != nil {
		return nil, err
	}
	if dashboard.Metadata.Project == "" {
		dashboard.Metadata.Project = project
	}
	return &dashboard, nil
}

// WithDefaultKind returns the JSON object with kind set when it is omitted.
func WithDefaultKind(raw json.RawMessage, kind string) (json.RawMessage, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return nil, err
	}
	if _, ok := fields["kind"]; ok {
		return raw, nil
	}
	if fields == nil {
		fields = map[string]json.RawMessage{}
	}
	fields["kind"] = json.RawMessage(strconv.Quote(kind))
	return json.Marshal(fields)
}

// MarshalText encodes a tool result as compact JSON without HTML escaping, so that messages such as PromQL
// comparisons stay readable by the model.
func MarshalText(value any) (string, error) {
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return "", err
	}
	return strings.TrimSuffix(buf.String(), "\n"), nil
}
