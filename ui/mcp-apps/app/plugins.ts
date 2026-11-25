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

import * as barChart from '@perses-dev/bar-chart-plugin';
import * as gaugeChart from '@perses-dev/gauge-chart-plugin';
import * as heatMapChart from '@perses-dev/heatmap-chart-plugin';
import * as histogramChart from '@perses-dev/histogram-chart-plugin';
import * as pieChart from '@perses-dev/pie-chart-plugin';
import type { PluginModuleResource } from '@perses-dev/plugin-system';
import { dynamicImportPluginLoader } from '@perses-dev/plugin-system';
import * as prometheus from '@perses-dev/prometheus-plugin';
import * as statChart from '@perses-dev/stat-chart-plugin';
import * as statusHistoryChart from '@perses-dev/status-history-chart-plugin';
import * as table from '@perses-dev/table-plugin';
import * as timeSeriesChart from '@perses-dev/timeseries-chart-plugin';
import * as timeSeriesTable from '@perses-dev/timeseries-table-plugin';

interface BundledModule {
  getPluginModule: () => unknown;
}

/**
 * The plugin modules bundled in the view: the Prometheus datasource and queries, and the panels that display time
 * series queries. Keep the panels in sync with panelKinds in pkg/tools/prometheus/panelkinds.go.
 */
const modules: BundledModule[] = [
  prometheus,
  timeSeriesChart,
  statChart,
  gaugeChart,
  barChart,
  pieChart,
  table,
  timeSeriesTable,
  heatMapChart,
  histogramChart,
  statusHistoryChart,
];

export const pluginLoader = dynamicImportPluginLoader(
  modules.map((module) => ({
    resource: module.getPluginModule() as PluginModuleResource,
    importPlugin: () => Promise.resolve(module),
  })),
);

/** The kinds of the bundled panel plugins. */
export function bundledPanelKinds(): string[] {
  return modules
    .flatMap((module) => (module.getPluginModule() as PluginModuleResource).spec.plugins)
    .filter((plugin) => plugin.kind === 'Panel')
    .map((plugin) => plugin.spec.name)
    .sort();
}
