/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import { test, expect } from '../../fixtures/consul-scenario.ts';

test.use({ scenario: 'peering' });

/**
 * Scenario: peering
 * Route: /ui/dc1/peers
 *
 * Asserts the peers list renders both seeded peerings with their real
 * status: `billing-cluster` (ACTIVE, one imported + one exported service)
 * and `reporting-cluster` (PENDING, a token generated but never established).
 * `Consul::Peer::List` renders each row's name as `[data-test-peer=<name>]`
 * and status as `Peerings::Badge` (`[data-test-peering-status="<lowercased-state>"]`,
 * an HDS badge whose text is the human state) — those are the stable hooks; the
 * badge's HDS-driven styling is not.
 */
test.describe('Peers list', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/ui/dc1/peers');
    await expect(page.locator('[data-test-peer]').first()).toBeVisible();
  });

  test('renders both seeded peers', async ({ page, consul }) => {
    const peerings = await consul.peerings();
    const names = peerings.map((p) => p.Name as string);
    expect(names).toEqual(expect.arrayContaining(['billing-cluster', 'reporting-cluster']));

    await expect(page.locator('[data-test-peer="billing-cluster"]')).toBeVisible();
    await expect(page.locator('[data-test-peer="reporting-cluster"]')).toBeVisible();
  });

  test('billing-cluster shows an Active status badge', async ({ page, consul }) => {
    const peering = await consul.peering('billing-cluster');
    expect(peering.State).toBe('ACTIVE');

    const row = page.locator('[data-test-tabular-row]', { has: page.locator('[data-test-peer="billing-cluster"]') });
    await expect(row.locator('[data-test-peering-status="active"]')).toHaveText('Active');
  });

  test('reporting-cluster shows a Pending status badge', async ({ page, consul }) => {
    const peering = await consul.peering('reporting-cluster');
    expect(peering.State).toBe('PENDING');

    const row = page.locator('[data-test-tabular-row]', {
      has: page.locator('[data-test-peer="reporting-cluster"]'),
    });
    await expect(row.locator('[data-test-peering-status="pending"]')).toHaveText('Pending');
  });

  test('imported/exported service counts reflect what was actually exported', async ({ page }) => {
    // payments is exported from the peer to the primary (1 imported), and
    // invoices is exported from the primary back to the peer (1 exported).
    const billingRow = page.locator('[data-test-tabular-row]', {
      has: page.locator('[data-test-peer="billing-cluster"]'),
    });
    await expect(billingRow.locator('[data-test-imported-count]')).toHaveText('1');
    await expect(billingRow.locator('[data-test-exported-count]')).toHaveText('1');

    // reporting-cluster never established, so it has neither imports nor exports.
    const reportingRow = page.locator('[data-test-tabular-row]', {
      has: page.locator('[data-test-peer="reporting-cluster"]'),
    });
    await expect(reportingRow.locator('[data-test-imported-count]')).toHaveText('0');
    await expect(reportingRow.locator('[data-test-exported-count]')).toHaveText('0');
  });
});
