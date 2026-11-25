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

import type { App, McpUiHostContext } from '@modelcontextprotocol/ext-apps';

import type { ToolResult } from './prometheusFetch';
import { structuredContentOf } from './prometheusFetch';
import type { RenderPanelsOutput } from './model';

/** What the view displays: waiting for the tool result, the panels, or why there are none. */
export type ViewState =
  | { kind: 'waiting' }
  | { kind: 'panels'; output: RenderPanelsOutput }
  | { kind: 'message'; message: string };

export interface HostState {
  view: ViewState;
  hostContext: McpUiHostContext;
  /** Whether the host lets the view call the tools of the server. Undefined until the view is connected. */
  serverTools?: boolean;
  /** The host is removing the view: it must stop refreshing. */
  tornDown?: boolean;
}

/** A minimal store for useSyncExternalStore, filled by the host notifications. */
export class HostStore {
  private state: HostState = { view: { kind: 'waiting' }, hostContext: {} };
  private listeners = new Set<() => void>();

  getState = (): HostState => this.state;

  subscribe = (listener: () => void): (() => void) => {
    this.listeners.add(listener);
    return () => this.listeners.delete(listener);
  };

  update(change: Partial<HostState>): void {
    this.state = { ...this.state, ...change };
    this.listeners.forEach((listener) => listener());
  }

  mergeHostContext(context: McpUiHostContext | undefined): void {
    if (context !== undefined) {
      this.update({ hostContext: { ...this.state.hostContext, ...context } });
    }
  }
}

/**
 * Attaches the store to the host notifications. Call it before App.connect: the host may send the tool result right
 * after the initialization, before React renders.
 */
export function attachHost(app: App, store: HostStore): void {
  // A host may reuse the view for another call: the panels of the previous one are removed.
  app.addEventListener('toolinput', () => store.update({ view: { kind: 'waiting' } }));
  app.addEventListener('toolresult', (result) => store.update({ view: viewOfResult(result as ToolResult) }));
  app.addEventListener('toolcancelled', (params) =>
    store.update({ view: { kind: 'message', message: `The tool call was cancelled${params.reason ? `: ${params.reason}` : '.'}` } }),
  );
  app.addEventListener('hostcontextchanged', (context) => store.mergeHostContext(context));
  app.onteardown = async () => {
    store.update({ tornDown: true });
    return {};
  };
}

/** Records what the host allows, once the view is connected. */
export function attachHostCapabilities(app: App, store: HostStore): void {
  store.mergeHostContext(app.getHostContext());
  store.update({ serverTools: app.getHostCapabilities()?.serverTools !== undefined });
}

// The messages of the results are written for the model, e.g. "Fix the queries and call the tool again": the view
// shows short messages for the user instead.
export const TOOL_ERROR_MESSAGE = 'The panels could not be rendered.';
export const NO_PANEL_MESSAGE = 'No panels: the queries failed or returned no data.';

/** Returns what the view displays for a result of perses_render_panels. */
export function viewOfResult(result: ToolResult): ViewState {
  if (result.isError) {
    return { kind: 'message', message: TOOL_ERROR_MESSAGE };
  }
  const output = structuredContentOf(result) as Partial<RenderPanelsOutput> | undefined;
  if (output === undefined) {
    return { kind: 'message', message: 'The panels could not be rendered: the tool result has no structured content.' };
  }
  if (output.status !== 'ok') {
    return { kind: 'message', message: NO_PANEL_MESSAGE };
  }
  if (!Array.isArray(output.panels)) {
    // The text content, which has no panel definitions: the host didn't pass the structured content.
    return { kind: 'message', message: 'The panels could not be rendered: the host did not pass their definitions to the view.' };
  }
  if (output.panels.length === 0) {
    return { kind: 'message', message: NO_PANEL_MESSAGE };
  }
  return { kind: 'panels', output: output as RenderPanelsOutput };
}
