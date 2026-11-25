import { resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

import react from '@vitejs/plugin-react';
import { defineConfig } from 'vitest/config';
import { viteSingleFile } from 'vite-plugin-singlefile';

const root = fileURLToPath(new URL('.', import.meta.url));

// The view is served by the MCP server as a single HTML resource: the scripts, styles and fonts are inlined, so that
// the view needs no network access (no CSP exception).
export default defineConfig({
  root: resolve(root, 'app'),
  base: './',
  plugins: [react(), viteSingleFile()],
  resolve: {
    // The Perses packages default-import the CommonJS icons of mdi-material-ui, whose default export is then the
    // module object: use the ES modules of the icons instead.
    alias: [{ find: /^mdi-material-ui\/(?!esm\/)(.+)$/, replacement: 'mdi-material-ui/esm/$1' }],
  },
  build: {
    outDir: resolve(root, 'dist'),
    // dist/.placeholder.html is committed: it keeps the embed pattern of ui/ui.go valid when the views are not built.
    emptyOutDir: false,
    assetsInlineLimit: Number.MAX_SAFE_INTEGER,
    chunkSizeWarningLimit: 10_000,
    rollupOptions: { input: resolve(root, 'app', 'panels.html') },
  },
  test: {
    root,
    environment: 'jsdom',
    include: ['app/**/*.test.{ts,tsx}'],
    setupFiles: ['app/testSetup.ts'],
    // The Perses packages import CommonJS files without extension (e.g. lodash/merge): Vite resolves them.
    server: { deps: { inline: [/@perses-dev\//] } },
  },
});
