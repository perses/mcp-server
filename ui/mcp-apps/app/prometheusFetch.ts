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

import type { FetchFn } from '@perses-dev/client';
import { UserFriendlyError } from '@perses-dev/client';

import type { DatasourceRef } from './model';
import { QUERY_DATA_TOOL } from './model';

/** The subset of a CallToolResult read by the bridge. */
export interface ToolResult {
  isError?: boolean;
  content?: Array<{ type: string; text?: string }>;
  structuredContent?: unknown;
}

/** Calls a tool of the MCP server through the host (App.callServerTool). */
export type CallServerTool = (
  params: { name: string; arguments: Record<string, unknown> },
  options?: { signal?: AbortSignal },
) => Promise<ToolResult>;

/**
 * Returns the fetch function of the Prometheus datasource of the view. The view can't reach Prometheus (the CSP of
 * MCP Apps hosts blocks network requests, and the view has no Perses credentials): the instant and range queries
 * of the Prometheus plugin are sent to the app-only tool perses_query_prometheus_data instead, which runs them
 * through the Perses proxy and returns the response of Prometheus. The other endpoints are not available.
 *
 * Errors are thrown as UserFriendlyError, like the fetch of Perses, so that the panels display their message.
 */
export function createPrometheusFetch(callServerTool: CallServerTool, datasource: DatasourceRef): FetchFn {
  return async (input, init) => {
    const url = new URL(input instanceof Request ? input.url : String(input), 'http://perses-mcp-app.invalid');
    const params = new URLSearchParams(url.search);
    for (const [key, value] of bodyParams(init?.body)) {
      params.append(key, value);
    }
    let args: Record<string, unknown>;
    if (url.pathname.endsWith('/api/v1/query_range')) {
      args = {
        query: params.get('query') ?? '',
        start: params.get('start') ?? '',
        end: params.get('end') ?? '',
        step: params.get('step') ?? '',
      };
    } else if (url.pathname.endsWith('/api/v1/query')) {
      args = { query: params.get('query') ?? '' };
      const time = params.get('time');
      if (time !== null) {
        args.time = time;
      }
    } else {
      throw new UserFriendlyError(`${url.pathname} is not available in the MCP App view`, 404);
    }
    args.datasource = datasource.project ? { name: datasource.name, project: datasource.project } : { name: datasource.name };

    const signal = init?.signal ?? undefined;
    signal?.throwIfAborted();
    let result: ToolResult;
    try {
      result = await callServerTool({ name: QUERY_DATA_TOOL, arguments: args }, { signal });
    } catch (error) {
      signal?.throwIfAborted();
      throw new UserFriendlyError(error instanceof Error ? error.message : String(error), 503);
    }
    if (result.isError) {
      throw new UserFriendlyError(resultText(result) || 'the query failed', 422);
    }
    const response = structuredContentOf(result);
    if (response?.data === undefined) {
      throw new UserFriendlyError('the host did not pass the data of the query to the view', 502);
    }
    return new Response(JSON.stringify(response), {
      status: 200,
      headers: { 'Content-Type': 'application/json' },
    });
  };
}

/**
 * Returns the structured content of a tool result. Some hosts only pass the content to the views: the JSON of the
 * text content is then used, when there is one.
 */
export function structuredContentOf(result: ToolResult): Record<string, unknown> | undefined {
  if (isObject(result.structuredContent)) {
    return result.structuredContent;
  }
  for (const content of result.content ?? []) {
    if (content.type !== 'text' || !content.text?.trimStart().startsWith('{')) {
      continue;
    }
    try {
      const parsed: unknown = JSON.parse(content.text);
      if (isObject(parsed)) {
        return parsed;
      }
    } catch {
      // Not JSON: a message for the model.
    }
  }
  return undefined;
}

function isObject(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function bodyParams(body: BodyInit | null | undefined): URLSearchParams {
  if (body instanceof URLSearchParams) {
    return body;
  }
  if (typeof body === 'string') {
    return new URLSearchParams(body);
  }
  return new URLSearchParams();
}

function resultText(result: ToolResult): string {
  return (result.content ?? [])
    .filter((content) => content.type === 'text' && content.text)
    .map((content) => content.text)
    .join('\n');
}
