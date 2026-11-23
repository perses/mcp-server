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

import { describe, expect, it } from 'vitest';

import { createDatasourceApi, DATASOURCE_PROXY_URL } from './datasourceApi';

describe('createDatasourceApi', () => {
  it('resolves Prometheus selectors to the datasource of the tool result, with its scrape interval', async () => {
    const api = createDatasourceApi({ name: 'prom' }, '15s');
    const resource = await api.getGlobalDatasource({ kind: 'PrometheusDatasource', name: 'other' });
    expect(resource?.metadata.name).toBe('prom');
    expect(resource?.spec.plugin.spec).toEqual({
      proxy: { kind: 'HTTPProxy', spec: { url: DATASOURCE_PROXY_URL } },
      scrapeInterval: '15s',
    });
    expect(await api.getGlobalDatasource({ kind: 'LokiDatasource' })).toBeUndefined();
  });

  it('leaves the scrape interval to the plugin default when the result has none', async () => {
    const resource = await createDatasourceApi({ name: 'prom' }).getGlobalDatasource({ kind: 'PrometheusDatasource' });
    expect(resource?.spec.plugin.spec).not.toHaveProperty('scrapeInterval');
  });
});
