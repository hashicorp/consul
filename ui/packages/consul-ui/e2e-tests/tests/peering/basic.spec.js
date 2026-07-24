/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

const { test, expect } = require('./fixtures');

/**
 * Peering - Basic Tests
 *
 * Single-instance tests that verify the Peers list page and the
 * "Add peer connection" modal UI without requiring a second Consul agent.
 */

test.describe('Peering - Basic', () => {
  test('peers list page loads', async ({ peeringsPage }) => {
    await peeringsPage.goto();
    await expect(peeringsPage.heading).toBeVisible({ timeout: 15000 });
  });

  test('"Add peer connection" button is visible on list page', async ({ peeringsPage }) => {
    await peeringsPage.goto();
    await expect(peeringsPage.heading).toBeVisible({ timeout: 15000 });
    await expect(peeringsPage.addPeerButton).toBeVisible({ timeout: 10000 });
  });

  test('"Add peer connection" modal opens with Generate token tab active', async ({
    peeringsPage,
  }) => {
    await peeringsPage.goto();
    await expect(peeringsPage.heading).toBeVisible({ timeout: 15000 });

    await peeringsPage.addPeerButton.click();

    await expect(peeringsPage.generateTokenTab).toBeVisible({ timeout: 10000 });
    await expect(peeringsPage.establishPeeringTab).toBeVisible();
    await expect(peeringsPage.peerNameInput).toBeVisible();
    await expect(peeringsPage.generateTokenButton).toBeVisible();
  });

  test('"Establish peering" tab shows token input', async ({ peeringsPage }) => {
    await peeringsPage.goto();
    await expect(peeringsPage.heading).toBeVisible({ timeout: 15000 });

    await peeringsPage.addPeerButton.click();
    await expect(peeringsPage.generateTokenTab).toBeVisible({ timeout: 10000 });

    await peeringsPage.establishPeeringTab.click();

    await expect(peeringsPage.peerNameInput).toBeVisible({ timeout: 10000 });
    await expect(peeringsPage.tokenTextarea).toBeVisible();
    await expect(peeringsPage.addPeerSubmitButton).toBeVisible();
  });

  test('API-created peer appears in the list', async ({ peeringsPage, acceptorApi }) => {
    await peeringsPage.goto();
    await expect(peeringsPage.heading).toBeVisible({ timeout: 15000 });

    // Generate a token via API to create a pending peer entry on this side.
    const peerName = `e2e-peer-${Date.now()}`;
    await acceptorApi.generateToken(peerName);

    await peeringsPage.page.reload();
    await peeringsPage.waitForPeerInList(peerName);
  });

  test('peer row links to the peer detail page', async ({ peeringsPage, acceptorApi }) => {
    await peeringsPage.goto();
    await expect(peeringsPage.heading).toBeVisible({ timeout: 15000 });

    const peerName = `e2e-peer-${Date.now()}`;
    await acceptorApi.generateToken(peerName);

    await peeringsPage.page.reload();
    await peeringsPage.waitForPeerInList(peerName);

    await peeringsPage.peerRow(peerName).click();
    await expect(peeringsPage.page).toHaveURL(new RegExp(`/peers/${peerName}`), {
      timeout: 15000,
    });
  });
});
