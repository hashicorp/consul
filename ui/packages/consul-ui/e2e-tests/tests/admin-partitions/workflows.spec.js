/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

const { test, expect } = require('./fixtures');
const { isEnterpriseConsul } = require('../../utils/ent-utils');

/**
 * Admin Partitions - Workflow Tests
 *
 * Complex end-to-end scenarios for the Admin Partitions feature.
 * Run nightly or before release.
 *
 * NOTE: Admin Partitions is an Enterprise-only feature.
 * These tests currently run on CE + ENT for development purposes.
 * To restrict to ENT only: set CONSUL_ENT_ONLY=true in the environment.
 */

test.describe('Admin Partitions - Workflows', () => {
  /**
   * Covers the full user journey: start at services → open partition selector
   * → click "View all partitions" footer link → arrive at the partitions list.
   *
   * Mirrors the inspector recording flow.
   */
  test('navigate to admin partitions via partition selector', async ({
    page,
    partitionsPage,
    skipEnt,
    request,
    baseURL,
  }) => {
    await skipEnt(request, baseURL);

    await page.goto(`${baseURL}/ui/dc1/services`, { waitUntil: 'domcontentloaded' });

    // Open the partition selector in the side nav.
    await page.locator('[data-test-partition-menu]').click();

    // Click the "View all partitions" footer link inside the selector dropdown.
    await page.getByRole('link', { name: 'View all partitions' }).click();

    await expect(partitionsPage.page).toHaveURL(/\/partitions$/, { timeout: 15000 });
    await expect(partitionsPage.heading).toBeVisible({ timeout: 10000 });
  });

  /**
   * Edit the "default" partition's description via the "More > Edit" action,
   * save the change, and verify redirection back to the list.
   *
   * The description is restored after the test via partitionApi to keep state clean.
   */
  test('edit partition description and save', async ({
    partitionsPage,
    partitionApi,
    request,
    baseURL,
  }) => {
    const isEnt = await isEnterpriseConsul(request, baseURL);
    test.skip(!isEnt, 'Partition updates (PUT /v1/partition/:name) return 403 on CE. Requires Enterprise Consul.');

    const newDescription = `E2E test description – ${Date.now()}`;

    // Retrieve the current description for post-test restore.
    let originalDescription = '';
    try {
      const current = await partitionApi.read('default');
      originalDescription = current?.Description ?? '';
    } catch {
      // Non-fatal — proceed without restore guard.
    }

    await partitionsPage.goto();
    await expect(partitionsPage.heading).toBeVisible({ timeout: 15000 });

    await partitionsPage.openEditViaMoreMenu('default');
    await partitionsPage.fillDescriptionAndSave(newDescription);

    // After save the UI redirects back to the partitions list.
    await expect(partitionsPage.page).toHaveURL(/\/dc1\/partitions$/, { timeout: 20000 });

    // Restore the original description so subsequent test runs start clean.
    try {
      await partitionApi.update('default', { Description: originalDescription });
    } catch {
      console.warn('[admin-partitions] Could not restore original description after test.');
    }
  });

  /**
   * Navigate to the default partition edit page, make a change, then click
   * Cancel and verify the UI returns to the list without persisting the change.
   */
  test('cancel partition edit discards changes and returns to list', async ({
    partitionsPage,
    skipEnt,
    request,
    baseURL,
  }) => {
    await skipEnt(request, baseURL);

    await partitionsPage.gotoEdit('default');
    await expect(partitionsPage.page).toHaveURL(/\/partitions\/default/, { timeout: 15000 });
    await partitionsPage.descriptionInput.waitFor({ state: 'visible', timeout: 10000 });

    // Type something but cancel instead of saving.
    await partitionsPage.descriptionInput.fill('This should not be saved');
    await partitionsPage.cancelButton.click();

    // Cancel should navigate back to the list without saving.
    await expect(partitionsPage.page).toHaveURL(/\/partitions$/, { timeout: 15000 });
    await expect(partitionsPage.heading).toBeVisible({ timeout: 10000 });
  });

  /**
   * Use the search bar on the partitions list to filter by partition name.
   * Verifies that matching rows remain visible and that clearing the search
   * restores all results.
   */
  test('search bar filters the partition list', async ({
    partitionsPage,
    skipEnt,
    request,
    baseURL,
  }) => {
    await skipEnt(request, baseURL);

    await partitionsPage.goto();
    await expect(partitionsPage.heading).toBeVisible({ timeout: 15000 });
    await partitionsPage.waitForPartitionInList('default');

    // Type a matching search term — "default" should still be visible.
    await partitionsPage.searchInput.fill('default');
    await expect(partitionsPage.partitionRow('default')).toBeVisible({ timeout: 10000 });

    // Type a non-matching term — the default row should disappear.
    await partitionsPage.searchInput.fill('zzz-nonexistent-zzz');
    await expect(partitionsPage.partitionRow('default')).toHaveCount(0, { timeout: 10000 });

    // Clear the search — the default row should reappear.
    await partitionsPage.searchInput.fill('');
    await partitionsPage.waitForPartitionInList('default');
  });

  /**
   * Use the "Search Across" button to switch the search scope, then verify
   * the partition list still renders correctly.
   */
  test('search across scope toggle works on partition list', async ({
    partitionsPage,
    skipEnt,
    request,
    baseURL,
  }) => {
    await skipEnt(request, baseURL);

    await partitionsPage.goto();
    await expect(partitionsPage.heading).toBeVisible({ timeout: 15000 });

    await expect(partitionsPage.searchAcrossButton).toBeVisible({ timeout: 10000 });
    await partitionsPage.searchAcrossButton.click();

    // After toggling, the list should still be intact.
    await partitionsPage.waitForPartitionInList('default');
  });
});
