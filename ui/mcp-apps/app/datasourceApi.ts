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

import type { DatasourceApi, GlobalDatasourceResource } from '@perses-dev/client';

import type { DatasourceRef } from './model';

/** The proxy URL of the datasource in the view. The requests never leave the view: see createPrometheusFetch. */
export const DATASOURCE_PROXY_URL = '/perses-mcp/prometheus';

/**
 * Returns the datasource API of the view: every Prometheus datasource selector resolves to the datasource of the
 * tool result, as the panels only reference it.
 */
export function createDatasourceApi(datasource: DatasourceRef): DatasourceApi {
  const resource: GlobalDatasourceResource = {
    kind: 'GlobalDatasource',
    metadata: { name: datasource.name },
    spec: {
      default: true,
      plugin: {
        kind: 'PrometheusDatasource',
        spec: { proxy: { kind: 'HTTPProxy', spec: { url: DATASOURCE_PROXY_URL } } },
      },
    },
  };
  const matches = (kind?: string): boolean => kind === undefined || kind === 'PrometheusDatasource';
  return {
    buildProxyUrl: () => DATASOURCE_PROXY_URL,
    getDatasource: async () => undefined,
    getGlobalDatasource: async (selector) => (matches(selector.kind) ? resource : undefined),
    listDatasources: async () => [],
    listGlobalDatasources: async (pluginKind) => (matches(pluginKind) ? [resource] : []),
  };
}
