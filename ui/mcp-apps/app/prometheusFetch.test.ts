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

import { UserFriendlyError } from '@perses-dev/client';
import { describe, expect, it, vi } from 'vitest';

import type { CallServerTool } from './prometheusFetch';
import { createPrometheusFetch } from './prometheusFetch';

const prometheusResponse = { status: 'success', data: { resultType: 'matrix', result: [] } };

function bridge(result: Awaited<ReturnType<CallServerTool>> = { structuredContent: prometheusResponse }) {
  const callServerTool = vi.fn<CallServerTool>().mockResolvedValue(result);
  const fetch = createPrometheusFetch(callServerTool, { name: 'prom', project: 'demo' });
  return { callServerTool, fetch };
}

describe('createPrometheusFetch', () => {
  it('sends the range queries of the form body to the data tool', async () => {
    const { callServerTool, fetch } = bridge();
    const response = await fetch('/perses-mcp/prometheus/api/v1/query_range?x=1', {
      method: 'POST',
      body: new URLSearchParams({ query: 'up', start: '100', end: '200', step: '15' }),
    });
    expect(callServerTool).toHaveBeenCalledWith(
      {
        name: 'perses_query_prometheus_data',
        arguments: { query: 'up', start: '100', end: '200', step: '15', datasource: { name: 'prom', project: 'demo' } },
      },
      { signal: undefined },
    );
    expect(response.status).toBe(200);
    expect(await response.json()).toEqual(prometheusResponse);
  });

  it('sends the instant queries of the URL to the data tool', async () => {
    const { callServerTool, fetch } = bridge();
    await fetch('/perses-mcp/prometheus/api/v1/query?query=up&time=150');
    expect(callServerTool.mock.calls[0]?.[0].arguments).toEqual({
      query: 'up',
      time: '150',
      datasource: { name: 'prom', project: 'demo' },
    });
  });

  it('returns the tool errors as user friendly errors', async () => {
    const { fetch } = bridge({ isError: true, content: [{ type: 'text', text: 'bad_data: parse error' }] });
    const error = await fetch('/p/api/v1/query?query=up(').catch((e: unknown) => e);
    expect(error).toBeInstanceOf(UserFriendlyError);
    expect((error as Error).message).toBe('bad_data: parse error');
  });

  it('rejects the other endpoints without calling the tool', async () => {
    const { callServerTool, fetch } = bridge();
    await expect(fetch('/p/api/v1/labels', { method: 'POST' })).rejects.toThrow('/p/api/v1/labels is not available');
    expect(callServerTool).not.toHaveBeenCalled();
  });

  it('does not call the tool when the request is aborted', async () => {
    const { callServerTool, fetch } = bridge();
    const controller = new AbortController();
    controller.abort();
    await expect(fetch('/p/api/v1/query?query=up', { signal: controller.signal })).rejects.toThrow();
    expect(callServerTool).not.toHaveBeenCalled();
  });
});
