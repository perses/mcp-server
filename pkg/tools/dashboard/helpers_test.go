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

package dashboard

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

// describe formats the optional context of a check: a format string and its arguments, or values to print.
func describe(context []any) string {
	if len(context) == 0 {
		return ""
	}
	if format, ok := context[0].(string); ok {
		return fmt.Sprintf(format, context[1:]...) + ": "
	}
	return fmt.Sprint(context...) + ": "
}

func equal(t testing.TB, expected, actual any, context ...any) {
	t.Helper()
	if !reflect.DeepEqual(expected, actual) {
		t.Errorf("%sexpected %#v, got %#v", describe(context), expected, actual)
	}
}

// has tells whether a string contains a substring, a map a key, or a slice an element.
func has(container, item any) bool {
	if text, ok := container.(string); ok {
		return strings.Contains(text, fmt.Sprint(item))
	}
	value := reflect.ValueOf(container)
	switch value.Kind() {
	case reflect.Map:
		return value.MapIndex(reflect.ValueOf(item)).IsValid()
	case reflect.Slice, reflect.Array:
		for i := range value.Len() {
			if reflect.DeepEqual(value.Index(i).Interface(), item) {
				return true
			}
		}
	}
	return false
}

func contains(t testing.TB, container, item any, context ...any) {
	t.Helper()
	if !has(container, item) {
		t.Errorf("%s%#v does not contain %#v", describe(context), container, item)
	}
}

func notContains(t testing.TB, container, item any, context ...any) {
	t.Helper()
	if has(container, item) {
		t.Errorf("%s%#v contains %#v", describe(context), container, item)
	}
}

// length returns the length of a string, map or slice, or -1.
func length(value any) int {
	v := reflect.ValueOf(value)
	switch v.Kind() {
	case reflect.String, reflect.Map, reflect.Slice, reflect.Array:
		return v.Len()
	}
	return -1
}

func empty(t testing.TB, value any) {
	t.Helper()
	if value != nil && length(value) != 0 {
		t.Errorf("expected empty, got %#v", value)
	}
}

func hasLen(t testing.TB, value any, expected int, context ...any) bool {
	t.Helper()
	if actual := length(value); actual != expected {
		t.Errorf("%sexpected %d items, got %d: %#v", describe(context), expected, actual, value)
		return false
	}
	return true
}
