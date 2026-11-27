/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import { test, expect } from '../../fixtures/consul-scenario.ts';

test.use({ scenario: 'peering' });

/**
 * Scenario: peering
 * Route: /ui/dc1/peers/billing-cluster
 *
 * `billing-cluster` is the ACTIVE peering established from the seeder, set
 * up so all three peer detail tabs have real data:
 *   - Imported Services (the default tab the route redirects fresh entries
 *     to, `dc.peers.show.imported` -> /imported-services) lists `payments`,
 *     exported from the PEER cluster via an `exported-services` config
 *     entry.
 *   - Exported Services (/exported-services) lists `invoices`, exported
 *     from the PRIMARY to the peer the same way.
 *   - Server Addresses (/addresses) lists the peer's `PeerServerAddresses`.
 *     This tab only renders at all when the local peering record has a
 *     `PeerID` (consul-ui's `Peer#isReceiver`, `models/peer.js`) — i.e. only
 *     on the side that called `/v1/peering/establish`. The seeder makes the
 *     PRIMARY that side specifically so this tab has something to show —
 *     see the doc comment at the top of `seed.ts`.
 *
 * `TabNav` renders each HDS tab (role="tab") with `data-test-tab="tab_<slugified-label>"`;
 * tabs are reached by clicking (a client-side transition), matching how
 * service-mesh's specs reach their tabs.
 */
const DETAIL = '/ui/dc1/peers/billing-cluster';

test.describe('Peer detail — billing-cluster', () => {
  test('peering is ACTIVE, established from the primary, with addresses (seed sanity check)', async ({ consul }) => {
    const peering = (await consul.peering('billing-cluster')) as { State: string; PeerID?: string; PeerServerAddresses?: string[] };
    expect(peering.State).toBe('ACTIVE');
    expect(peering.PeerID).toBeTruthy();
    expect(peering.PeerServerAddresses?.length).toBeGreaterThan(0);
  });

  test('header and status card render the peer name and Active status', async ({ page }) => {
    await page.goto(DETAIL);
    await expect(page.getByRole('heading', { name: 'billing-cluster', exact: true })).toBeVisible();
    await expect(page.locator('[data-test-peering-status="active"]')).toHaveText('Active');
  });

  test('all three peer tabs are present, defaulting to Imported Services', async ({ page }) => {
    await page.goto(DETAIL);
    await expect(page).toHaveURL(/\/imported-services$/);
    await expect(page.locator('[data-test-tab="tab_imported-services"]')).toBeVisible();
    await expect(page.locator('[data-test-tab="tab_exported-services"]')).toBeVisible();
    await expect(page.locator('[data-test-tab="tab_server-addresses"]')).toBeVisible();
  });

  test('Imported Services lists the payments service exported from the peer', async ({ page }) => {
    await page.goto(DETAIL);
    await expect(page).toHaveURL(/\/imported-services$/);

    await expect(page.locator('[data-test-service-name]', { hasText: 'payments' })).toBeVisible();
  });

  test('Exported Services lists the invoices service exported to the peer', async ({ page }) => {
    await page.goto(DETAIL);
    await page.locator('[data-test-tab="tab_exported-services"]').click();
    await expect(page).toHaveURL(/\/exported-services$/);

    await expect(page.locator('[data-test-service-name]', { hasText: 'invoices' })).toBeVisible();
  });

  test('Server Addresses lists the peer\'s server address', async ({ page, consul }) => {
    const peering = (await consul.peering('billing-cluster')) as { PeerServerAddresses?: string[] };
    const [address] = peering.PeerServerAddresses ?? [];
    expect(address).toBeTruthy();

    await page.goto(DETAIL);
    await page.locator('[data-test-tab="tab_server-addresses"]').click();
    await expect(page).toHaveURL(/\/addresses$/);

    await expect(page.getByText(address!, { exact: true })).toBeVisible();
  });
});
