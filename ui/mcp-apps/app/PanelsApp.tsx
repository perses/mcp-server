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
import { Alert, Box, CssBaseline, IconButton, Stack, ThemeProvider, Tooltip, Typography } from '@mui/material';
import { FetchProvider } from '@perses-dev/client';
import { ChartsProvider, generateChartsTheme, getTheme, SnackbarProvider, TimeZoneProvider } from '@perses-dev/components';
import { DatasourceStoreProvider, Panel, VariableProvider } from '@perses-dev/dashboards';
import {
  DataQueriesProvider,
  PluginRegistry,
  TimeRangeControls,
  TimeRangeProvider,
  usePlugin,
  useSuggestedStepMs,
} from '@perses-dev/plugin-system';
import type { DurationString, PanelDefinition, TimeRangeValue } from '@perses-dev/spec';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import FullscreenIcon from 'mdi-material-ui/Fullscreen';
import FullscreenExitIcon from 'mdi-material-ui/FullscreenExit';
import type { ReactElement } from 'react';
import { useEffect, useMemo, useRef, useState, useSyncExternalStore } from 'react';
import useResizeObserver from 'use-resize-observer';

import { createDatasourceApi } from './datasourceApi';
import type { HostStore } from './hostStore';
import type { RenderPanelsOutput } from './model';
import { timeRangeContext, toTimeRangeValue } from './model';
import { pluginLoader } from './plugins';
import { createPrometheusFetch } from './prometheusFetch';

const queryClient = new QueryClient({ defaultOptions: { queries: { refetchOnWindowFocus: false, retry: 0 } } });

/** Panels showing one or a few values are lower than the charts and tables. */
const COMPACT_KINDS = new Set(['StatChart', 'GaugeChart']);

export interface PanelsAppProps {
  app: App;
  store: HostStore;
}

export function PanelsApp({ app, store }: PanelsAppProps): ReactElement {
  const { view, hostContext, serverTools, tornDown } = useSyncExternalStore(store.subscribe, store.getState);
  const mode = hostContext.theme === 'dark' ? 'dark' : 'light';
  const muiTheme = useMemo(() => getTheme(mode), [mode]);
  const chartsTheme = useMemo(() => generateChartsTheme(muiTheme, {}), [muiTheme]);
  const insets = hostContext.safeAreaInsets;

  return (
    <ThemeProvider theme={muiTheme}>
      <CssBaseline />
      <ChartsProvider chartsTheme={chartsTheme}>
        <SnackbarProvider anchorOrigin={{ vertical: 'bottom', horizontal: 'right' }} variant="default" content="">
          <PluginRegistry
            pluginLoader={pluginLoader}
            defaultPluginKinds={{ Panel: 'TimeSeriesChart', TimeSeriesQuery: 'PrometheusTimeSeriesQuery' }}
          >
            <QueryClientProvider client={queryClient}>
              <Box
                sx={{
                  paddingTop: `${8 + (insets?.top ?? 0)}px`,
                  paddingRight: `${8 + (insets?.right ?? 0)}px`,
                  paddingBottom: `${8 + (insets?.bottom ?? 0)}px`,
                  paddingLeft: `${8 + (insets?.left ?? 0)}px`,
                }}
              >
                {view.kind === 'panels' && serverTools !== false && (
                  <PanelsDashboard app={app} output={view.output} hostContext={hostContext} paused={tornDown === true} />
                )}
                {view.kind === 'panels' && serverTools === false && (
                  <Alert severity="warning">
                    This client does not let the view call the tools of the MCP server: the data of the panels cannot be
                    loaded.
                  </Alert>
                )}
                {view.kind === 'message' && <Alert severity="warning">{view.message}</Alert>}
                {view.kind === 'waiting' && (
                  <Typography variant="body2" color="text.secondary">
                    Loading the panels…
                  </Typography>
                )}
              </Box>
            </QueryClientProvider>
          </PluginRegistry>
        </SnackbarProvider>
      </ChartsProvider>
    </ThemeProvider>
  );
}

interface PanelsDashboardProps {
  app: App;
  output: RenderPanelsOutput;
  hostContext: McpUiHostContext;
  /** Stops the automatic refresh. */
  paused: boolean;
}

function PanelsDashboard({ app, output, hostContext, paused }: PanelsDashboardProps): ReactElement {
  const [timeRange, setTimeRange] = useState<TimeRangeValue>(() => toTimeRangeValue(output.timeRange));
  const [refreshInterval, setRefreshInterval] = useState<DurationString>('0s');
  const [timeZone, setTimeZone] = useState(hostContext.timeZone ?? 'local');
  const fetchFn = useMemo(
    () => createPrometheusFetch((params, options) => app.callServerTool(params, options), output.datasource),
    [app, output.datasource],
  );
  const datasourceApi = useMemo(
    () => createDatasourceApi(output.datasource, output.scrapeInterval),
    [output.datasource, output.scrapeInterval],
  );

  // The automatic refresh stops while the view is scrolled out of the chat.
  const container = useRef<HTMLDivElement>(null);
  const [visible, setVisible] = useState(true);
  useEffect(() => {
    if (container.current === null || typeof IntersectionObserver === 'undefined') {
      return;
    }
    const observer = new IntersectionObserver((entries) => setVisible(entries.some((entry) => entry.isIntersecting)));
    observer.observe(container.current);
    return (): void => observer.disconnect();
  }, []);

  // The model is told when the user changes the time range, not for the time range of its own call.
  const initialTimeRange = useRef(timeRange);
  useEffect(() => {
    if (timeRange === initialTimeRange.current || app.getHostCapabilities()?.updateModelContext === undefined) {
      return;
    }
    app
      .updateModelContext({ content: [{ type: 'text', text: timeRangeContext(output, timeRange) }] })
      .catch(() => undefined); // The host may refuse: the panels don't depend on it.
  }, [app, output, timeRange]);

  const isFullscreen = hostContext.displayMode === 'fullscreen';
  const canToggleFullscreen = hostContext.availableDisplayModes?.includes('fullscreen') ?? false;
  const toggleFullscreen = (): void => {
    app.requestDisplayMode({ mode: isFullscreen ? 'inline' : 'fullscreen' }).catch(() => undefined);
  };

  return (
    <FetchProvider fetchFn={fetchFn}>
      <TimeZoneProvider timeZone={timeZone}>
        <TimeRangeProvider
          timeRange={timeRange}
          setTimeRange={setTimeRange}
          refreshInterval={visible && !paused ? refreshInterval : '0s'}
          setRefreshInterval={setRefreshInterval}
        >
          <VariableProvider>
            <DatasourceStoreProvider datasourceApi={datasourceApi}>
              <Stack direction="row" alignItems="center" justifyContent="flex-end" gap={1} sx={{ marginBottom: 1 }}>
                <TimeRangeControls
                  timeZone={timeZone}
                  onTimeZoneChange={(option) => setTimeZone(option.value)}
                  showZoomButtons={false}
                />
                {canToggleFullscreen && (
                  <Tooltip title={isFullscreen ? 'Exit fullscreen' : 'Fullscreen'}>
                    <IconButton size="small" onClick={toggleFullscreen} aria-label="toggle fullscreen">
                      {isFullscreen ? <FullscreenExitIcon /> : <FullscreenIcon />}
                    </IconButton>
                  </Tooltip>
                )}
              </Stack>
              <Box
                ref={container}
                sx={{
                  display: 'grid',
                  gap: 1,
                  gridTemplateColumns: output.panels.length === 1 ? '1fr' : 'repeat(auto-fill, minmax(360px, 1fr))',
                }}
              >
                {output.panels.map((panel, index) => (
                  <PanelItem key={index} definition={panel} />
                ))}
              </Box>
              {output.warnings?.map((warning) => (
                <Typography key={warning} variant="caption" color="text.secondary" component="p" sx={{ marginTop: 1 }}>
                  {warning}
                </Typography>
              ))}
            </DatasourceStoreProvider>
          </VariableProvider>
        </TimeRangeProvider>
      </TimeZoneProvider>
    </FetchProvider>
  );
}

/** A panel with its queries, as GridItemContent of @perses-dev/dashboards renders it. */
function PanelItem({ definition }: { definition: PanelDefinition }): ReactElement {
  const { ref, width } = useResizeObserver<HTMLDivElement>();
  const suggestedStepMs = useSuggestedStepMs(width);
  const { data: plugin } = usePlugin('Panel', definition.spec.plugin.kind);
  const pluginSpec = definition.spec.plugin.spec;
  // Memoized: new options would make DataQueriesProvider run the queries again.
  const pluginQueryOptions = useMemo(
    () => (typeof plugin?.queryOptions === 'function' ? plugin.queryOptions(pluginSpec) : plugin?.queryOptions),
    [plugin, pluginSpec],
  );
  const options = useMemo(() => ({ suggestedStepMs, ...pluginQueryOptions }), [suggestedStepMs, pluginQueryOptions]);
  const height = COMPACT_KINDS.has(definition.spec.plugin.kind) ? 200 : 320;

  return (
    <Box ref={ref} sx={{ height, minWidth: 0 }}>
      <DataQueriesProvider definitions={definition.spec.queries ?? []} options={options}>
        <Panel definition={definition} />
      </DataQueriesProvider>
    </Box>
  );
}
