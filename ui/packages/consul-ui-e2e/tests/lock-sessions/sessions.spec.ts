/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import { test, expect } from '../../fixtures/consul-scenario.ts';

test.use({ scenario: 'lock-sessions' });

/**
 * Scenario: lock-sessions
 * Route: /ui/dc1/nodes/node-primary/lock-sessions
 *
 * Asserts the Lock Sessions tab renders the seeded sessions and their columns.
 */
const ROUTE = '/ui/dc1/nodes/node-primary/lock-sessions';

test.describe('Lock sessions', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto(ROUTE);
    await expect(page.locator('[data-test-session-name]').first()).toBeVisible();
  });

  test('renders the seeded sessions', async ({ page, consul }) => {
    const seeded = await consul.nodeSessions('node-primary');
    expect(seeded.length).toBeGreaterThan(0);
    expect(await page.locator('[data-test-session-name]').count()).toBeGreaterThan(0);
  });

  test('shows behavior for each session', async ({ page }) => {
    const behaviors = page.locator('[data-test-session-behavior]');
    await expect(behaviors.first()).toBeVisible();
    const text = (await behaviors.first().textContent())?.trim();
    expect(['release', 'delete']).toContain(text);
  });

  test('exposes an invalidate action', async ({ page }) => {
    await expect(page.locator('[data-test-delete]').first()).toBeVisible();
  });
});
