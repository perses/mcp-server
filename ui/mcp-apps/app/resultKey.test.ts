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

import { renderHook } from '@testing-library/react';
import { describe, expect, it } from 'vitest';

import { useResultKey } from './resultKey';

describe('useResultKey', () => {
  it('changes with the result, not with the renders', () => {
    const first = { panels: [] };
    const second = { panels: [] };
    const { result, rerender } = renderHook((value: object | undefined) => useResultKey(value), {
      initialProps: first as object | undefined,
    });
    const firstKey = result.current;
    rerender(first);
    expect(result.current).toBe(firstKey);
    rerender(second);
    expect(result.current).not.toBe(firstKey);
    rerender(undefined);
    expect(result.current).toBe(0);
  });
});
