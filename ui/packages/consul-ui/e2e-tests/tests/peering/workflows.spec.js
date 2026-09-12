/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

const { test, expect } = require('./fixtures');

/**
 * Peering - Workflow Tests
 *
 * Two-instance tests that drive both Consul UIs simultaneously:
 *
 *  Instance A (primary)  →  http://localhost:4200 (local) / :8500 (CI)
 *  Instance B (peer)     →  http://localhost:8501 (local + CI)
 *
 * Test scenario (mirrors the manual test plan):
 *  1. Instance A: navigate to Peers → "Add peer connection" → generate token
 *  2. Instance B: navigate to Peers → "Add peer connection" → paste token
 *     → "Establish peering" → "Add Peer"
 *  3. Verify state transitions: Pending (acceptor after token gen) → Active
 *  4. View peer detail on both sides — check status + server addresses copyable
 *  5. Delete from Instance A → verify removal on both sides
 *
 * Requires both Consul agents running (ports 8500/4200 and 8501).
 * Override with PLAYWRIGHT_PEER_URL env var (default: http://localhost:8501).
 */

test.describe('Peering - Workflows', () => {
  /**
   * Full establish-and-delete lifecycle driven through both UIs.
   *
   * Steps mirror the manual test plan:
   *  1. Instance A UI: generate token (no server address) → copy it
   *  2. Instance A: peer shows Pending immediately after token generation
   *  3. Instance B UI: paste token on Establish tab → Add Peer
   *  4. Both sides transition to Active
   *  5. View peer detail on both sides: status card shows Active,
   *     Addresses tab has copyable server address buttons
   *  6. Delete from Instance A → row disappears; Instance B eventually clears
   */
  test.skip('establish and delete a peer connection', async ({
    peeringsPage,
    peerPeeringsPage,
    acceptorApi,
    dialerApi,
  }) => {
    const ts = Date.now();
    const acceptorPeerName = `e2e-a-${ts}`;
    const dialerPeerName = `e2e-b-${ts}`;

    acceptorApi.track(acceptorPeerName);
    dialerApi.track(dialerPeerName);

    // ── Step 1: Instance A — generate token via the UI ─────────────────────
    await peeringsPage.goto();
    await expect(peeringsPage.heading).toBeVisible({ timeout: 15000 });

    await peeringsPage.addPeerButton.click();
    await expect(peeringsPage.generateTokenTab).toBeVisible({ timeout: 10000 });
    await peeringsPage.peerNameInput.fill(acceptorPeerName);

    // Generate token — form switches to the token display screen.
    await peeringsPage.generateTokenButton.click();
    await expect(peeringsPage.copyTokenButton).toBeVisible({ timeout: 15000 });

    // Read the raw token from the CopyableCode <code> element.
    const peeringToken = await peeringsPage.page
      .locator('.copyable-code pre code')
      .first()
      .textContent({ timeout: 10000 });
    expect(peeringToken.trim().length, 'Peering token should not be empty').toBeGreaterThan(0);

    // ── Step 2: Instance A — verify Pending state ──────────────────────────
    await peeringsPage.closeModalButton.click();
    await peeringsPage.page.reload();
    await peeringsPage.waitForPeerInList(acceptorPeerName);

    const acceptorRow = peeringsPage.peerListRow(acceptorPeerName);

    await expect(acceptorRow.locator('.peerings-badge__text')).toHaveText('Pending', {
      timeout: 10000,
    });

    // ── Step 3: Instance B — establish via the UI ──────────────────────────
    await peerPeeringsPage.goto();
    await expect(peerPeeringsPage.heading).toBeVisible({ timeout: 15000 });

    await peerPeeringsPage.establishPeeringViaUI(dialerPeerName, peeringToken.trim());

    await peerPeeringsPage.page.reload();
    await peerPeeringsPage.waitForPeerInList(dialerPeerName);

    // ── Step 4: Both sides — wait for Active ──────────────────────────────
    // Instance A: poll until Active. Use 60s — the handshake over the binary UI
    // at port 8501 can take longer than 30s to propagate back to the acceptor.
    await expect(async () => {
      await peeringsPage.page.reload();
      await peeringsPage.waitForPeerInList(acceptorPeerName);
      await expect(acceptorRow.locator('.peerings-badge__text')).toHaveText('Active');
    }).toPass({ timeout: 60000 });

    // Instance B: same.
    const dialerRow = peerPeeringsPage.peerListRow(dialerPeerName);

    await expect(async () => {
      await peerPeeringsPage.page.reload();
      await peerPeeringsPage.waitForPeerInList(dialerPeerName);
      await expect(dialerRow.locator('.peerings-badge__text')).toHaveText('Active');
    }).toPass({ timeout: 60000 });

    // ── Step 5: View peer detail on both sides ─────────────────────────────
    // Instance A detail page — status card and addresses tab.
    await peeringsPage.peerRow(acceptorPeerName).click();
    await expect(peeringsPage.page).toHaveURL(new RegExp(`/peers/${acceptorPeerName}`), {
      timeout: 15000,
    });
    await expect(peeringsPage.page.locator('.peerings-badge__text')).toHaveText('Active', {
      timeout: 10000,
    });

    // Addresses tab: at least one address has a copy button.
    await peeringsPage.page
      .getByRole('navigation', { name: 'Secondary' })
      .getByRole('link', { name: 'Server Addresses' })
      .click();
    await expect(peeringsPage.page.locator('button[aria-label*="Address"]').first()).toBeVisible({
      timeout: 10000,
    });

    // Instance B detail page.
    await peerPeeringsPage.peerRow(dialerPeerName).click();
    await expect(peerPeeringsPage.page).toHaveURL(new RegExp(`/peers/${dialerPeerName}`), {
      timeout: 15000,
    });
    await expect(peerPeeringsPage.page.locator('.peerings-badge__text')).toHaveText('Active', {
      timeout: 10000,
    });

    await peerPeeringsPage.page
      .getByRole('navigation', { name: 'Secondary' })
      .getByRole('link', { name: 'Server Addresses' })
      .click();
    await expect(
      peerPeeringsPage.page.locator('button[aria-label*="Address"]').first()
    ).toBeVisible({ timeout: 10000 });

    // ── Step 6: Delete from Instance A ─────────────────────────────────────
    await peeringsPage.goto();
    await peeringsPage.waitForPeerInList(acceptorPeerName);

    await acceptorRow.getByRole('button', { name: 'More' }).click();
    await peeringsPage.page.getByRole('menuitem', { name: 'Delete' }).click();
    // The last "Delete" button is the inline confirmation action.
    await peeringsPage.page.getByRole('button', { name: 'Delete' }).last().click();

    // Instance A: row disappears (Deleting → removed).
    await expect(peeringsPage.peerRow(acceptorPeerName)).toHaveCount(0, { timeout: 20000 });

    // Instance B: peer eventually goes Terminated then disappears.
    await expect(async () => {
      await peerPeeringsPage.page.reload();
      await expect(peerPeeringsPage.peerRow(dialerPeerName)).toHaveCount(0);
    }).toPass({ timeout: 30000 });

    // Skip cleanup — removed through the UI above.
    acceptorApi._trackedNames = acceptorApi._trackedNames.filter((n) => n !== acceptorPeerName);
    dialerApi._trackedNames = dialerApi._trackedNames.filter((n) => n !== dialerPeerName);
  });

  /**
   * API-path smoke test: establish via API and verify Active state on both UIs.
   * Faster CI signal without a full UI walkthrough.
   */
  test('API-established peer shows Active on both instances', async ({
    peeringsPage,
    peerPeeringsPage,
    acceptorApi,
    dialerApi,
  }) => {
    const ts = Date.now();
    const acceptorPeerName = `e2e-api-a-${ts}`;
    const dialerPeerName = `e2e-api-b-${ts}`;

    const peeringToken = await acceptorApi.generateToken(acceptorPeerName);
    await dialerApi.establish(dialerPeerName, peeringToken);

    await expect(async () => {
      await peeringsPage.goto();
      await peeringsPage.waitForPeerInList(acceptorPeerName);
      await expect(
        peeringsPage.peerListRow(acceptorPeerName).locator('.peerings-badge__text')
      ).toHaveText('Active');
    }).toPass({ timeout: 30000 });

    await expect(async () => {
      await peerPeeringsPage.goto();
      await peerPeeringsPage.waitForPeerInList(dialerPeerName);
      await expect(
        peerPeeringsPage.peerListRow(dialerPeerName).locator('.peerings-badge__text')
      ).toHaveText('Active');
    }).toPass({ timeout: 60000 });
  });

  /**
   * View peer imported and exported services.
   *
   * Prerequisites: both peered Consul instances must have services configured
   * with exported-services config entries (as set up by consul-testing-repo).
   *
   * Steps:
   *  1. Establish a peering via API.
   *  2. On Instance B (importer): navigate to peer detail → Imported Services tab
   *     → at least one service name is visible.
   *  3. On Instance A (exporter): navigate to peer detail → Exported Services tab
   *     → at least one service name is visible.
   */
  test('view peer imported and exported services', async ({
    peeringsPage,
    peerPeeringsPage,
    acceptorApi,
    dialerApi,
  }) => {
    const ts = Date.now();
    const acceptorPeerName = `e2e-svc-a-${ts}`;
    const dialerPeerName = `e2e-svc-b-${ts}`;

    // Establish the peering via API so the test focuses on service visibility.
    const peeringToken = await acceptorApi.generateToken(acceptorPeerName);
    await dialerApi.establish(dialerPeerName, peeringToken);

    // Wait until both sides are Active before exporting services.
    await expect(async () => {
      await peeringsPage.goto();
      await peeringsPage.waitForPeerInList(acceptorPeerName);
      await expect(
        peeringsPage.peerListRow(acceptorPeerName).locator('.peerings-badge__text')
      ).toHaveText('Active');
    }).toPass({ timeout: 30000 });

    await expect(async () => {
      await peerPeeringsPage.goto();
      await peerPeeringsPage.waitForPeerInList(dialerPeerName);
      await expect(
        peerPeeringsPage.peerListRow(dialerPeerName).locator('.peerings-badge__text')
      ).toHaveText('Active');
    }).toPass({ timeout: 60000 });

    // Add this test peer to the exported-services config on Instance A.
    // This makes the pre-existing 'billing' service visible as an imported
    // service on Instance B.  The fixture teardown restores the original config.
    await acceptorApi.addPeerToExportedServices(acceptorPeerName);

    // ── Instance B (importer): Imported Services tab ────────────────────────
    // Poll until the imported service propagates (config change takes a moment).
    // Instance B (8501) runs the release binary's old UI (.consul-service-list);
    // this branch's UI uses the migrated .consul-service-table. Match either,
    // plus data-test-service-name (stripped in prod, so also match by href).
    const serviceNameSel =
      '[data-test-service-name], ' +
      '.consul-service-table a[href*="/services/"], ' +
      '.consul-service-list a[href*="/services/"]';

    await expect(async () => {
      await peerPeeringsPage.goto();
      await peerPeeringsPage.peerRow(dialerPeerName).click();
      await expect(peerPeeringsPage.page).toHaveURL(new RegExp(`/peers/${dialerPeerName}`), {
        timeout: 5000,
      });
      await peerPeeringsPage.page
        .getByRole('navigation', { name: 'Secondary' })
        .getByRole('link', { name: 'Imported Services' })
        .click();
      await expect(peerPeeringsPage.page.locator(serviceNameSel).first()).toBeVisible({
        timeout: 5000,
      });
    }).toPass({ timeout: 30000 });

    // ── Instance A (exporter): Exported Services tab ────────────────────────
    // Exported services template uses <a class="hds-typography-display-300">
    // (not inside consul-service-list), so include that as a fallback.
    const exportedServiceNameSel =
      '[data-test-service-name], a.hds-typography-display-300[href*="/services/"]';

    await peeringsPage.peerRow(acceptorPeerName).click();
    await expect(peeringsPage.page).toHaveURL(new RegExp(`/peers/${acceptorPeerName}`), {
      timeout: 15000,
    });

    await peeringsPage.page
      .getByRole('navigation', { name: 'Secondary' })
      .getByRole('link', { name: 'Exported Services' })
      .click();

    await expect(peeringsPage.page.locator(exportedServiceNameSel).first()).toBeVisible({
      timeout: 15000,
    });
  });
});
