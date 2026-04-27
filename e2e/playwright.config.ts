import { defineConfig, devices } from '@playwright/test'

const API_URL = process.env.API_URL ?? 'http://localhost:8080'
const UI_URL  = process.env.UI_URL  ?? 'http://localhost:3000'

export default defineConfig({
  testDir: './tests',

  // Run tests sequentially — later suites depend on state from earlier ones
  fullyParallel: false,
  workers: 1,

  // Fail fast so a broken global setup surfaces immediately
  forbidOnly: !!process.env.CI,
  retries: 0,

  reporter: [
    ['list'],
    ['html', { outputFolder: 'playwright-report', open: 'never' }],
  ],

  globalSetup:    './global-setup.ts',
  globalTeardown: './global-teardown.ts',

  timeout: 30_000,

  use: {
    // UI tests navigate the Next.js frontend
    baseURL: UI_URL,
    actionTimeout: 10_000,
    // Save a screenshot on failure
    screenshot: 'only-on-failure',
    // Reuse the authenticated browser session created in global-setup
    storageState: './e2e/.auth.json',
  },

  projects: [
    {
      name: 'api-tests',
      testMatch: /0[1-5]-.*\.spec\.ts/,
      use: {
        // API tests don't need a browser — use request context only
        storageState: undefined,
      },
    },
    {
      name: 'ui-tests',
      testMatch: /06-.*\.spec\.ts/,
      use: {
        ...devices['Desktop Chrome'],
        storageState: './e2e/.auth.json',
      },
    },
  ],
})

export { API_URL, UI_URL }
