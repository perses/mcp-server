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

import { App } from '@modelcontextprotocol/ext-apps';
import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';

import { attachHost, attachHostCapabilities, HostStore } from './hostStore';
import { PanelsApp } from './PanelsApp';

const store = new HostStore();
const app = new App(
  { name: 'Perses panels', version: '1.0.0' },
  { availableDisplayModes: ['inline', 'fullscreen'] },
  { autoResize: true },
);
// The handlers are attached before connecting: the host sends the tool result right after the initialization.
attachHost(app, store);

const root = createRoot(document.getElementById('root') as HTMLElement);
root.render(
  <StrictMode>
    <PanelsApp app={app} store={store} />
  </StrictMode>,
);

app
  .connect()
  .then(() => attachHostCapabilities(app, store))
  .catch((error: unknown) => {
    store.update({
      view: { kind: 'message', message: `Could not connect to the host: ${error instanceof Error ? error.message : String(error)}` },
    });
  });
