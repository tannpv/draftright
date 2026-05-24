import { defineConfig } from '@playwright/test';

// E2E runs the production build of the admin (so it picks up .env.production →
// VITE_API_URL=https://api.draftright.info) on a local preview server, then
// drives a real Chromium against it. This exercises the real in-browser image
// compression in the Report-a-Bug flow.
export default defineConfig({
  testDir: './e2e',
  timeout: 90_000,
  expect: { timeout: 20_000 },
  fullyParallel: false,
  retries: 0,
  use: {
    baseURL: 'http://localhost:4173',
    headless: true,
    screenshot: 'only-on-failure',
  },
  webServer: {
    command: 'npm run build && npm run preview -- --port 4173 --strictPort',
    url: 'http://localhost:4173',
    reuseExistingServer: true,
    timeout: 240_000,
  },
});
