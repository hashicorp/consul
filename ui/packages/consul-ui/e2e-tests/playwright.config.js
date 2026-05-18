/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

const { defineConfig, devices } = require('@playwright/test');

/**
 * Playwright Configuration for Consul UI E2E Tests
 *
 * Two-tier test structure:
 * - basic: Fast, essential tests (run on every PR)
 * - workflows: Complex scenarios (run nightly)
 */

module.exports = defineConfig({
  // Test directory
  testDir: './tests',

  // Output directory for test results
  outputDir: './reports/test-results',

  // Run tests in files in parallel
  fullyParallel: true,

  // Fail the build on CI if you accidentally left test.only in the source code
  forbidOnly: !!process.env.CI,

  // Retry on CI only
  retries: process.env.CI ? 2 : 0,

  // Opt out of parallel tests on CI
  workers: process.env.CI ? 1 : undefined,

  // Reporter to use
  reporter: process.env.CI
    ? [
        ['html', { outputFolder: './reports/html-report', open: 'never' }],
        ['junit', { outputFile: './reports/junit/results.xml' }],
        ['json', { outputFile: './reports/json/results.json' }],
        ['list'],
        ['github'],
      ]
    : [
        ['html', { outputFolder: './reports/html-report', open: 'never' }],
        ['junit', { outputFile: './reports/junit/results.xml' }],
        ['json', { outputFile: './reports/json/results.json' }],
        ['list'],
      ],

  // Shared settings for all projects
  use: {
    // Base URL for navigation
    // CI: defaults to port 8500 (Consul server)
    // Local: defaults to port 4200 (Ember dev server)
    // Can be overridden by PLAYWRIGHT_BASE_URL or BASE_URL env var
    baseURL:
      process.env.PLAYWRIGHT_BASE_URL ||
      process.env.BASE_URL ||
      (process.env.CI ? 'http://localhost:8500' : 'http://localhost:4200'),

    // Use saved authentication state
    storageState: 'e2e-tests/auth-state.json',

    // Collect trace on first retry
    trace: 'on-first-retry',

    // Screenshot on failure
    screenshot: 'only-on-failure',

    // Video on failure
    video: 'retain-on-failure',

    // Action timeout
    actionTimeout: 10000,

    // Navigation timeout
    navigationTimeout: 30000,
  },

  // Configure projects for major browsers
  projects: [
    {
      name: 'basic',
      testMatch: ['**/basic/**/*.spec.js', '**/basic.spec.js'],
      use: {
        ...devices['Desktop Chrome'],
      },
      fullyParallel: true,
      retries: 1,
      timeout: 30000, // 30s per test
    },

    {
      name: 'all',
      testMatch: ['**/*.spec.js'],
      use: {
        ...devices['Desktop Chrome'],
      },
      fullyParallel: false,
      retries: 1,
      timeout: 60000, // 60s per test
    },

    {
      name: 'workflows',
      testMatch: ['**/workflows/**/*.spec.js', '**/workflows.spec.js'],
      use: {
        ...devices['Desktop Chrome'],
      },
      fullyParallel: false, // Sequential for cross-DC state
      retries: 2,
      timeout: 60000, // 60s per test
      dependencies: ['basic'], // Only run if basic tests pass
    },
  ],

  // Global setup and teardown
  globalSetup: require.resolve('./global-setup.js'),
  globalTeardown: require.resolve('./global-teardown.js'),

  // Web server configuration (if needed)
  // webServer: {
  //   command: 'npm start',
  //   url: 'http://localhost:4200',
  //   reuseExistingServer: !process.env.CI,
  //   timeout: 120000,
  // },
});
