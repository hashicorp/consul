/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import { defineConfig, devices } from '@playwright/test';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { AGENT_ADDR } from './lib/consul-client.ts';
import { loadDotEnv } from './lib/env.ts';

loadDotEnv();

const __dirname = path.dirname(fileURLToPath(import.meta.url));

const CONSUL_UI_DIR = path.resolve(
  __dirname,
  process.env.CONSUL_UI_DIR ?? '../consul-ui',
);

// Which UI is under test:
//   ember (default) — the working-tree Ember dev server on :4200, proxied to the agent.
//   agent           — the UI the agent itself serves on :8500, i.e. a Consul binary
//                     built with the UI embedded (`make ui-regen`). This is what CI uses.
const UI_SOURCE = process.env.UI_SOURCE ?? 'ember';
if (UI_SOURCE !== 'ember' && UI_SOURCE !== 'agent') {
  throw new Error(`UI_SOURCE must be "ember" or "agent" (got "${UI_SOURCE}")`);
}
const BASE_URL =
  process.env.E2E_BASE_URL || (UI_SOURCE === 'agent' ? AGENT_ADDR : 'http://localhost:4200');

// Which Consul edition this run tests. Tests tagged @ent only run on `ent`,
// tests tagged @ce only on `ce`, untagged tests on both. The agent under test
// must be the same edition (checked in fixtures/consul-scenario.ts).
const EDITION = process.env.EDITION ?? 'ce';
if (EDITION !== 'ce' && EDITION !== 'ent') {
  throw new Error(`EDITION must be "ce" or "ent" (got "${EDITION}")`);
}

/**
 * Each spec file picks its scenario with `test.use({ scenario })`; the
 * scenario fixture starts, seeds and stops it. Every scenario uses the same
 * :8500, so files run one worker at a time. With UI_SOURCE=ember, `webServer`
 * also starts the working-tree Ember UI on :4200, proxied to :8500.
 */
export default defineConfig({
  testDir: './tests',
  fullyParallel: false,
  workers: 1,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  reporter: process.env.CI ? [['github'], ['html', { open: 'never' }]] : 'html',

  use: {
    baseURL: BASE_URL,
    trace: 'on-first-retry',
    screenshot: 'only-on-failure',
  },

  projects: [
    {
      name: EDITION,
      grepInvert: EDITION === 'ce' ? /@ent\b/ : /@ce\b/,
      metadata: { edition: EDITION },
      use: { ...devices['Desktop Chrome'] },
    },
  ],

  // UI_SOURCE=ember: start the consul-ui Ember dev server, proxied to the
  // Consul agent on :8500. Mirrors `pnpm start:consul` in the
  // consul-ui package. UI_SOURCE=agent needs no web server.
  webServer:
    UI_SOURCE === 'ember'
      ? {
          command: `pnpm --dir "${CONSUL_UI_DIR}" start:consul`,
          url: `${BASE_URL}/ui/`,
          timeout: 180_000,
          reuseExistingServer: !process.env.CI,
          env: {
            CONSUL_HTTP_ADDR: AGENT_ADDR,
            EMBER_SERVE_PORT: '4200',
          },
        }
      : undefined,

  metadata: { edition: EDITION, uiSource: UI_SOURCE },
});
