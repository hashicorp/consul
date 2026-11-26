/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import type { Page } from '@playwright/test';
import { test, expect } from '../../fixtures/consul-scenario.ts';

test.use({ scenario: 'service-mesh' });

/**
 * Scenario: service-mesh
 * Route: /ui/dc1/services/web-variants/instances/node-primary/web-variants
 *
 * Exercises the mutually-exclusive branches in the Upstreams table
 * (`consul::upstream-instance::table`, one `<tr data-test-tabular-row>` per
 * upstream):
 *   - LocalBindSocketPath set  -> the Address cell shows the socket path and the
 *                                 Mode cell shows its mode (Consul forbids an
 *                                 upstream carrying both a socket path and a
 *                                 bind port, so this is the only way the
 *                                 socket-path branch is reached).
 *   - Datacenter != current dc -> the Datacenter cell shows it; with
 *                                 LocalBindPort set the Address cell shows the
 *                                 bind address too.
 *
 * The instance route redirects fresh entries to /health-checks, so the Upstreams
 * tab is reached by clicking it after the page loads.
 */
const INSTANCE = '/ui/dc1/services/web-variants/instances/node-primary/web-variants';

function row(page: Page, name: string) {
  return page.locator('[data-test-tabular-row]').filter({
    has: page.locator('[data-test-upstream-name]', { hasText: new RegExp(`^\\s*${name}\\s*$`) }),
  });
}

test.beforeEach(async ({ page }) => {
  await page.goto(INSTANCE);
  await page.locator('[data-test-tab="tab_upstreams"]').click();
  await expect(page).toHaveURL(/\/upstreams$/);
});

test.describe('Service mesh — upstream detail branches', () => {
  test('socket-path upstream shows socket path + mode instead of an address', async ({ page }) => {
    const db = row(page, 'db');
    await expect(db.locator('[data-test-upstream-address]')).toContainText('/tmp/db.sock');
    await expect(db.locator('[data-test-upstream-mode]')).toHaveText('0700');
  });

  test('cross-datacenter upstream shows the Datacenter row and its bind address', async ({ page }) => {
    const cache = row(page, 'cache');
    await expect(cache.locator('[data-test-upstream-datacenter]')).toHaveText('dc2');
    await expect(cache.locator('[data-test-upstream-address]')).toContainText('127.0.0.1:9291');
    // Not a socket-path upstream, so no mode.
    await expect(cache.locator('[data-test-upstream-mode]')).toHaveText('-');
  });
});
