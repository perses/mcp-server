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

import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';

import { describe, expect, it } from 'vitest';

import { bundledPanelKinds } from './plugins';

describe('bundledPanelKinds', () => {
  it('are the panel kinds accepted by perses_render_panels', () => {
    const goSource = readFileSync(resolve(import.meta.dirname, '../../../pkg/tools/prometheus/panelkinds.go'), 'utf8');
    const panelKinds = goSource.slice(goSource.indexOf('var panelKinds = '), goSource.indexOf('\n}\n', goSource.indexOf('var panelKinds = ')));
    const goKinds = [...panelKinds.matchAll(/^\t"(\w+)": \{$/gm)].map((match) => match[1]).sort();
    expect(goKinds.length).toBeGreaterThan(0);
    expect(bundledPanelKinds()).toEqual(goKinds);
  });
});
