/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import { test, expect } from '../../fixtures/consul-scenario.ts';

test.use({ scenario: 'mesh-gateway' });

/**
 * Scenario: mesh-gateway
 * Route: /ui/dc1/services/mesh-gateway/instances/node-primary/mesh-gateway/addresses
 *
 * The Addresses tab only appears for Kind=mesh-gateway and lists the service's
 * TaggedAddresses as cards. Selectors are the consul-ui `data-test-address-*`
 * hooks from the HDS migration.
 */
const INSTANCE = '/ui/dc1/services/mesh-gateway/instances/node-primary/mesh-gateway';

test.describe('Service mesh gateway — Addresses tab', () => {
  test('registered as a mesh-gateway with tagged addresses', async ({ consul }) => {
    const svc = await consul.agentService('mesh-gateway');
    expect(svc.Kind).toBe('mesh-gateway');
    expect(Object.keys((svc.TaggedAddresses ?? {}) as object).length).toBeGreaterThan(0);
  });

  test('Addresses tab is available and renders address cards', async ({ page }) => {
    await page.goto(INSTANCE);
    await page.getByRole('tab', { name: 'Addresses' }).click();
    await expect(page).toHaveURL(/addresses/);

    const cards = page.locator('[data-test-addresses] [data-test-address-card]');
    expect(await cards.count()).toBeGreaterThan(0);
    await expect(page.locator('[data-test-address-value]').first()).toBeVisible();
  });

  // UI bug: addresses.hbs compares against `route.model.item.Port`, which the
  // ServiceInstance model doesn't define, so "(default)" never renders. Re-enable
  // once the template compares against `item.Service.Port`.
  test.fixme('marks the address matching the instance as (default)', async ({ page }) => {
    await page.goto(`${INSTANCE}/addresses`);
    await expect(page.locator('[data-test-address-default]').first()).toBeVisible();
  });
});
