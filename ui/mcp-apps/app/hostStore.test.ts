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

import type { App } from '@modelcontextprotocol/ext-apps';

import { attachHost, attachHostCapabilities, HostStore, NO_PANEL_MESSAGE, TOOL_ERROR_MESSAGE, viewOfResult } from './hostStore';
import type { RenderPanelsOutput } from './model';
import { timeRangeContext, toTimeRangeValue } from './model';

const output = {
  status: 'ok',
  datasource: { name: 'prom' },
  timeRange: { pastDuration: '6h' },
  step: '1m',
  panels: [{ kind: 'Panel', spec: { plugin: { kind: 'TimeSeriesChart', spec: {} } } }],
};

describe('viewOfResult', () => {
  it('shows the panels of a successful result', () => {
    expect(viewOfResult({ structuredContent: output })).toEqual({ kind: 'panels', output });
  });

  it('shows a message for the user, not the message for the model, when no panel is displayed', () => {
    const message = 'Nothing was displayed. Fix the queries and call the tool again.';
    expect(viewOfResult({ structuredContent: { ...output, status: 'error', message, panels: [] } })).toEqual({
      kind: 'message',
      message: NO_PANEL_MESSAGE,
    });
    expect(viewOfResult({ structuredContent: { ...output, panels: [] } })).toEqual({ kind: 'message', message: NO_PANEL_MESSAGE });
  });

  it('reads the JSON of the text content when the host passes no structured content', () => {
    const text = (value: unknown) => ({ content: [{ type: 'text', text: JSON.stringify(value) }] });
    expect(viewOfResult(text(output))).toEqual({ kind: 'panels', output });
    expect(viewOfResult(text({ status: 'error', message: 'Nothing was displayed.' }))).toEqual({
      kind: 'message',
      message: NO_PANEL_MESSAGE,
    });
    // The text content of perses_render_panels has no panel definitions.
    const { panels: _panels, ...withoutPanels } = output;
    expect(viewOfResult(text(withoutPanels))).toMatchObject({ kind: 'message', message: expect.stringContaining('did not pass') });
    expect(viewOfResult({ content: [{ type: 'text', text: 'not JSON' }] })).toMatchObject({ kind: 'message' });
  });

  it('shows a message for the user on a tool error', () => {
    const view = viewOfResult({ isError: true, content: [{ type: 'text', text: 'invalid panels (fix spec, unit or kind)' }] });
    expect(view).toEqual({ kind: 'message', message: TOOL_ERROR_MESSAGE });
  });
});

describe('HostStore', () => {
  it('merges the host context changes and notifies the listeners', () => {
    const store = new HostStore();
    let notifications = 0;
    store.subscribe(() => notifications++);
    store.mergeHostContext({ theme: 'dark', timeZone: 'Europe/Berlin' });
    store.mergeHostContext({ theme: 'light' });
    expect(store.getState().hostContext).toEqual({ theme: 'light', timeZone: 'Europe/Berlin' });
    expect(notifications).toBe(2);
  });
});

/** A fake App that records the handlers of the view. */
function fakeApp(capabilities: Record<string, unknown> = {}) {
  const listeners = new Map<string, (params: unknown) => void>();
  const app = {
    addEventListener: (event: string, listener: (params: unknown) => void) => listeners.set(event, listener),
    getHostContext: () => ({ theme: 'dark' }),
    getHostCapabilities: () => capabilities,
    onteardown: undefined as undefined | (() => Promise<unknown>),
  };
  return { app: app as unknown as App, emit: (event: string, params: unknown) => listeners.get(event)?.(params), raw: app };
}

describe('attachHost', () => {
  it('removes the panels of the previous call on a new tool input', () => {
    const store = new HostStore();
    const { app, emit } = fakeApp();
    attachHost(app, store);
    emit('toolresult', { structuredContent: output });
    expect(store.getState().view.kind).toBe('panels');
    emit('toolinput', { arguments: {} });
    expect(store.getState().view).toEqual({ kind: 'waiting' });
  });

  it('stops the view on teardown', async () => {
    const store = new HostStore();
    const { app, raw } = fakeApp();
    attachHost(app, store);
    await raw.onteardown?.();
    expect(store.getState().tornDown).toBe(true);
  });

  it('records whether the host lets the view call the tools', () => {
    const store = new HostStore();
    attachHostCapabilities(fakeApp({ serverTools: {} }).app, store);
    expect(store.getState()).toMatchObject({ serverTools: true, hostContext: { theme: 'dark' } });
    attachHostCapabilities(fakeApp().app, store);
    expect(store.getState().serverTools).toBe(false);
  });
});

describe('timeRangeContext', () => {
  it('describes the panels and the new time range', () => {
    const panels = { ...output, panels: [{ kind: 'Panel', spec: { display: { name: 'CPU' }, plugin: { kind: 'StatChart', spec: {} } } }] };
    const rendered = panels as unknown as RenderPanelsOutput;
    expect(timeRangeContext(rendered, { pastDuration: '6h' })).toContain('panels "CPU" to the past 6h.');
    expect(
      timeRangeContext(rendered, { start: new Date('2026-01-02T11:00:00Z'), end: new Date('2026-01-02T12:00:00Z') }),
    ).toContain('to 2026-01-02T11:00:00.000Z to 2026-01-02T12:00:00.000Z.');
  });
});

describe('toTimeRangeValue', () => {
  it('converts relative and absolute time ranges', () => {
    expect(toTimeRangeValue({ pastDuration: '6h' })).toEqual({ pastDuration: '6h' });
    expect(toTimeRangeValue({ start: '2026-01-02T11:00:00Z', end: '2026-01-02T12:00:00Z' })).toEqual({
      start: new Date('2026-01-02T11:00:00Z'),
      end: new Date('2026-01-02T12:00:00Z'),
    });
    expect(toTimeRangeValue(undefined)).toEqual({ pastDuration: '1h' });
  });
});
