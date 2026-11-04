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

package dashboardutil

import (
	"errors"
	"net/http"

	"github.com/perses/perses/pkg/client/perseshttp"
)

// IsBadRequest reports whether the error is a 400 response of the Perses API, i.e. a validation failure, and
// returns the message of Perses unchanged.
func IsBadRequest(err error) (string, bool) {
	var requestErr *perseshttp.RequestError
	if !errors.As(err, &requestErr) || requestErr.StatusCode != http.StatusBadRequest {
		return "", false
	}
	message := requestErr.Message
	if message == "" {
		message = err.Error()
	}
	return message, true
}

// IsConflict reports whether the error is a 409 response of the Perses API: the resource already exists.
func IsConflict(err error) bool {
	var requestErr *perseshttp.RequestError
	return errors.As(err, &requestErr) && requestErr.StatusCode == http.StatusConflict
}

// IsNotFound reports whether the error is a 404 response of the Perses API.
func IsNotFound(err error) bool {
	var requestErr *perseshttp.RequestError
	return errors.As(err, &requestErr) && requestErr.StatusCode == http.StatusNotFound
}
