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

package ephemeraldashboard

import (
	"errors"
	"net/http"
	"testing"

	"github.com/perses/perses/pkg/client/perseshttp"
	"github.com/stretchr/testify/assert"
)

func TestExplainError(t *testing.T) {
	notAllowed := &perseshttp.RequestError{Message: "Method Not Allowed", StatusCode: http.StatusMethodNotAllowed}
	notFound := &perseshttp.RequestError{Message: "document not found", StatusCode: http.StatusNotFound}

	assert.ErrorContains(t, explainError(notAllowed, "error creating", false), "disabled on this Perses server")
	assert.ErrorContains(t, explainError(notFound, "error listing", false), "disabled on this Perses server")
	assert.ErrorContains(t, explainError(notFound, "error getting", true), "either it does not exist")

	other := errors.New("boom")
	assert.ErrorIs(t, explainError(other, "error getting", true), other)
}
