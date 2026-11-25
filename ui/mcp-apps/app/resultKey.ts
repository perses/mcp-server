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

import { useRef } from 'react';

/**
 * Returns a number that changes with result, an object of the store: a new tool result gets a new number, so that a
 * component keyed by it starts again, e.g. from the time range of that result, even when the host sends it to a view
 * that is already displayed.
 */
export function useResultKey(result: object | undefined): number {
  const keys = useRef(new WeakMap<object, number>());
  const next = useRef(0);
  if (result === undefined) {
    return 0;
  }
  let key = keys.current.get(result);
  if (key === undefined) {
    next.current += 1;
    key = next.current;
    keys.current.set(result, key);
  }
  return key;
}
