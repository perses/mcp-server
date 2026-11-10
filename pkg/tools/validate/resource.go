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

package validate

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/perses/perses/pkg/client/perseshttp"
)

// result is the result of the validation tools. Error is set when the resource is invalid.
type result struct {
	Valid bool   `json:"valid"`
	Error string `json:"error,omitempty"`
}

// invalid is the result of a resource that is rejected before reaching Perses, or that Perses rejects as invalid.
func invalid(err error) *result {
	return &result{Error: err.Error()}
}

// serverResult converts the response of the Perses validation API. A rejection of the resource (HTTP 400 with the
// message of Perses) is an invalid result carrying that message; any other error, such as an unreachable server, a
// server error or a 400 without a Perses message (e.g. from a proxy), is returned unchanged as a tool error since the
// resource could not be validated.
func serverResult(err error) (*result, error) {
	if err == nil {
		return &result{Valid: true}, nil
	}
	var requestErr *perseshttp.RequestError
	if errors.As(err, &requestErr) && requestErr.StatusCode == http.StatusBadRequest && requestErr.Message != "" {
		return invalid(errors.New(requestErr.Message)), nil
	}
	return nil, err
}

// kindOf returns the kind of a variable, which tells whether it is a project or a global resource. It is empty when
// the kind is missing or the variable cannot be decoded.
func kindOf(raw json.RawMessage) string {
	var header struct {
		Kind string `json:"kind"`
	}
	_ = json.Unmarshal(raw, &header)
	return header.Kind
}
