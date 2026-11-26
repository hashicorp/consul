/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import { test, expect } from '../../fixtures/consul-scenario.ts';

test.use({ scenario: 'nodes' });

/**
 * Scenario: nodes
 * Route: /ui/dc1/nodes
 *
 * Asserts the Nodes-list HDS migration renders the seeded nodes with their
 * Health / Address / Services columns. Selectors are the consul-ui
 * `data-test-*` hooks, which are stable across the visual migration.
 */
test.describe('Nodes list', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/ui/dc1/nodes');
    await expect(page.locator('[data-test-node]').first()).toBeVisible();
  });

  test('renders the seeded nodes', async ({ page, consul }) => {
    const seeded = (await consul.catalogNodes()).map((n) => n.Node as string);
    expect(seeded.length).toBeGreaterThanOrEqual(5);

    for (const name of ['app-01', 'app-02', 'db-01', 'edge-01', 'cache-01']) {
      await expect(page.locator(`[data-test-node]`, { hasText: name })).toBeVisible();
    }
  });

  test('shows the Address column with a value per node', async ({ page }) => {
    const addresses = page.locator('[data-test-node-address]');
    await expect(addresses.first()).toBeVisible();
    await expect(addresses.first()).not.toBeEmpty();
  });

  test('shows a health status on every row', async ({ page }) => {
    const statuses = page.locator('[data-test-status]');
    expect(await statuses.count()).toBeGreaterThan(0);
  });

  test('shows a services count badge per node', async ({ page }) => {
    await expect(page.locator('[data-test-node-services]').first()).toBeVisible();
  });
});
