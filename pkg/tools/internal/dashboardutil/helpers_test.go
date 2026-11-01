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
	"reflect"
	"strings"
	"testing"
)

func equal(t testing.TB, expected, actual any) bool {
	t.Helper()
	if !reflect.DeepEqual(expected, actual) {
		t.Errorf("expected %#v, got %#v", expected, actual)
		return false
	}
	return true
}

func errorContains(t testing.TB, err error, message string) {
	t.Helper()
	if err == nil || !strings.Contains(err.Error(), message) {
		t.Errorf("expected an error containing %q, got %v", message, err)
	}
}

func empty(t testing.TB, value any) {
	t.Helper()
	if value != nil && reflect.ValueOf(value).Len() != 0 {
		t.Errorf("expected empty, got %#v", value)
	}
}

func hasLen(t testing.TB, value any, expected int) bool {
	t.Helper()
	if actual := reflect.ValueOf(value).Len(); actual != expected {
		t.Errorf("expected %d items, got %d: %#v", expected, actual, value)
		return false
	}
	return true
}
