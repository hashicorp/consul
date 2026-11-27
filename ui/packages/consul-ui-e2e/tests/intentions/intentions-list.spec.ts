/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import { test, expect } from '../../fixtures/consul-scenario.ts';

test.use({ scenario: 'intentions' });

/**
 * Scenario: intentions
 * Route: /ui/dc1/intentions
 *
 * Asserts the Intentions list renders the seeded Allow / Deny / Application
 * Aware rows with their correct badges, the wildcard source label, the
 * Permissions count, and that the "Intention type" quick filter and search
 * box narrow the table. Selectors are the consul-ui `data-test-*` hooks,
 * which are stable across the HDS visual migration.
 */
test.describe('Intentions list', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/ui/dc1/intentions');
    await expect(page.locator('[data-test-intention]').first()).toBeVisible();
  });

  test('renders all three seeded intentions', async ({ page }) => {
    await expect(page.locator('[data-test-intention]')).toHaveCount(3);
  });

  test('shows the correct Action badge per intention', async ({ page }) => {
    await expect(page.locator('[data-test-intention-action="allow"]')).toHaveCount(1);
    await expect(page.locator('[data-test-intention-action="deny"]')).toHaveCount(1);
    // Application-Aware intentions have Action=undefined, and Ember omits an
    // attribute binding entirely for undefined — there's no
    // `data-test-intention-action=""` to match, only its *absence*.
    const appAwareRow = page
      .getByRole('row')
      .filter({ has: page.locator('[data-test-intention]') })
      .filter({ hasNot: page.locator('[data-test-intention-action]') });
    await expect(appAwareRow).toHaveCount(1);
    await expect(appAwareRow.getByText('App aware')).toBeVisible();
  });

  test('renders the wildcard source as "All Services (*)"', async ({ page }) => {
    await expect(page.locator('[data-test-intention-source="*"]', { hasText: 'All Services (*)' })).toBeVisible();
  });

  test('shows a Permissions count on the Layer 7 intention only', async ({ page }) => {
    // Only the web -> payments row is Application Aware (no Action attribute).
    const row = page
      .getByRole('row')
      .filter({ has: page.locator('[data-test-intention-destination="payments"]') })
      .filter({ hasNot: page.locator('[data-test-intention-action]') });
    await expect(row.getByText('1 Permission')).toBeVisible();
  });

  test('"Deny" quick filter narrows the table to the wildcard intention', async ({ page }) => {
    await page.locator('[data-test-access-filter="deny"]').click();
    await expect(page.getByRole('row').filter({ has: page.locator('[data-test-intention]') })).toHaveCount(1);
    await expect(page.locator('[data-test-intention-action="deny"]')).toBeVisible();
  });

  test('search narrows the table by service name', async ({ page }) => {
    await page.getByPlaceholder('Search').fill('api');
    await page.getByPlaceholder('Search').press('Enter');
    await expect(page.getByRole('row').filter({ has: page.locator('[data-test-intention]') })).toHaveCount(1);
    await expect(page.locator('[data-test-intention-destination="api"]')).toBeVisible();
  });
});
