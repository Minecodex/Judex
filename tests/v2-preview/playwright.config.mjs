import { fileURLToPath } from 'node:url';
import { defineConfig } from '@playwright/test';
export default defineConfig({
 testDir: '.', testMatch: 'v2.spec.mjs', timeout: 120000, workers: 1,
 use: { baseURL: 'http://127.0.0.1:5199', viewport: { width: 1280, height: 720 }, locale: 'zh-CN', trace: 'retain-on-failure' },
 outputDir: '../../.cache/v2-preview/test-results',
 webServer: { command: 'node tests/v2-preview/server.mjs', cwd: fileURLToPath(new URL('../../', import.meta.url)), url: 'http://127.0.0.1:5199', reuseExistingServer: true },
});
