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

import type { PanelDefinition, TimeRangeValue } from '@perses-dev/spec';

/** Names of the tools of the Perses MCP server used by the view. */
export const RENDER_PANELS_TOOL = 'perses_render_panels';
export const QUERY_DATA_TOOL = 'perses_query_prometheus_data';

export interface DatasourceRef {
  name: string;
  /** Set for a project datasource, undefined for a global datasource. */
  project?: string;
}

/** Relative (pastDuration, e.g. "1h") or absolute (RFC 3339 start and end) time range. */
export type PanelsTimeRange = { pastDuration: string } | { start: string; end: string };

/** The structured content of perses_render_panels (RenderPanelsOutput in pkg/tools/prometheus/render.go). */
export interface RenderPanelsOutput {
  status: 'ok' | 'error';
  message?: string;
  datasource: DatasourceRef;
  timeRange: PanelsTimeRange;
  step: string;
  panels: PanelDefinition[];
  warnings?: string[];
}

/**
 * Returns what the model is told when the user changes the time range of the panels, as its statistics are those of
 * the time range of the tool call.
 */
export function timeRangeContext(output: RenderPanelsOutput, timeRange: TimeRangeValue): string {
  const titles = output.panels.map((panel) => `"${panel.spec.display?.name ?? 'Untitled'}"`).join(', ');
  const range =
    'pastDuration' in timeRange
      ? `the past ${timeRange.pastDuration}`
      : `${timeRange.start.toISOString()} to ${timeRange.end.toISOString()}`;
  return (
    `The user changed the time range of the Perses panels ${titles} to ${range}. The statistics returned by ` +
    `${RENDER_PANELS_TOOL} are those of the previous time range: call it again for this one if they are needed.`
  );
}

/** Converts the time range of the tool result to the time range of Perses. */
export function toTimeRangeValue(timeRange: PanelsTimeRange | undefined): TimeRangeValue {
  if (timeRange !== undefined && 'start' in timeRange) {
    return { start: new Date(timeRange.start), end: new Date(timeRange.end) };
  }
  return { pastDuration: (timeRange?.pastDuration ?? '1h') as `${number}h` };
}
