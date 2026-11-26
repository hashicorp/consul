/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import { test, expect } from '../../fixtures/consul-scenario.ts';
import type { Page } from '@playwright/test';

test.use({ scenario: 'intentions' });

/**
 * Scenario: intentions
 * Route: /ui/dc1/intentions/create
 *
 * Drives the Layer 7 ("Application Aware") permission builder modal: path
 * validation, method restriction, headers, and the confirm-before-downgrade
 * warning when an existing Layer 7 intention is switched to Allow/Deny.
 */

/**
 * Pick `name` in the Source/Destination SuperSelectWithCreate at `wrapper`.
 * The trigger and its search input are both `role="combobox"` once expanded,
 * and the page also has a real `<input>` (Description) satisfying
 * `getByRole('textbox')` — `.last()` on the scoped combobox pair is what
 * actually reaches the search box.
 */
async function pickService(page: Page, wrapper: string, name: string) {
  const el = page.locator(wrapper);
  await el.getByRole('combobox').click();
  await el.getByRole('combobox').last().fill(name);
  await page.getByRole('option', { name, exact: true }).click();
}

test.describe('Layer 7 permissions', () => {
  // Remove what these tests create even when one fails part-way. Deleting a
  // missing intention or config entry is a no-op in Consul.
  test.afterEach(async ({ consul }) => {
    for (const destination of ['checkout', 'temp-downgrade-target']) {
      await consul.deleteIntention({ SourceName: 'api', DestinationName: destination });
    }
    await consul.deleteConfigEntry('service-defaults', 'temp-downgrade-target');
  });

  test('path without a leading "/" is rejected', async ({ page }) => {
    await page.goto('/ui/dc1/intentions/create');

    await pickService(page, '[data-test-source-element]', 'api');
    await pickService(page, '[data-test-destination-element]', 'checkout');
    await page.getByRole('radio', { name: /application aware/i }).click();

    await page.locator('[data-test-create-permission]').click();
    // `id="path-type-select"` in the template — the HDS SuperSelect's visible
    // label isn't reachable via getByLabel (no real for/id association).
    await page.locator('#path-type-select').click();
    await page.getByRole('option', { name: 'Prefixed by' }).click();
    await page.getByLabel('Prefixed by').fill('admin'); // no leading "/"

    await expect(page.getByText("Path should begin with a '/'")).toBeVisible();
    await expect(page.locator('[data-test-intention-permission-submit]')).toBeDisabled();
  });

  test('builds a permission with a path, restricted methods, and a header', async ({ page, consul }) => {
    await page.goto('/ui/dc1/intentions/create');

    await pickService(page, '[data-test-source-element]', 'api');
    await pickService(page, '[data-test-destination-element]', 'checkout');
    await page.getByRole('radio', { name: /application aware/i }).click();

    await page.locator('[data-test-create-permission]').click();
    const modal = page.getByRole('dialog', { name: 'Add permission' });
    // Scoped to the modal — the page's own Action radios also have a "Deny".
    await modal.getByRole('radio', { name: 'Deny' }).click();

    await page.locator('#path-type-select').click();
    await page.getByRole('option', { name: 'Prefixed by' }).click();
    await page.getByLabel('Prefixed by').fill('/reports');

    // Turning "All methods" off enables the per-method checkboxes.
    await modal.getByRole('switch', { name: 'All methods' }).click();
    await modal.getByRole('checkbox', { name: 'GET', exact: true }).check();

    const header = modal.getByRole('group', { name: 'Header', exact: true });
    await header.getByRole('textbox', { name: 'Header name' }).fill('X-Debug');
    await header.getByRole('textbox', { name: 'Header information' }).fill('true'); // default type: Exactly Matching
    await page.locator('[data-test-add-header]').click();
    await expect(modal.locator('[data-test-list-row]').filter({ hasText: 'X-Debug' })).toContainText('true');

    await page.locator('[data-test-intention-permission-submit]').click();

    // Permission card on the intention form. The modal stays mounted (just
    // hidden) after closing — same as the downgrade-warning dialog — so its
    // "GET" method checkbox label is still in the DOM; scope to the actual
    // permission card list (`[data-test-list-row]`) to avoid the collision.
    const permissionCard = page.locator('[data-test-list-row]');
    await expect(permissionCard.getByText('PathPrefix')).toBeVisible();
    await expect(permissionCard.getByText('/reports')).toBeVisible();
    await expect(permissionCard.getByText('GET')).toBeVisible();

    await page.getByRole('button', { name: 'Save' }).click();
    await expect(page).toHaveURL(/\/ui\/dc1\/intentions$/);
    await expect(page.getByText('Your intention has been saved.')).toBeVisible();

    const seeded = await consul.intentions();
    const created = seeded.find((i) => i.SourceName === 'api' && i.DestinationName === 'checkout') as
      | { Permissions?: Array<{ HTTP?: { PathPrefix?: string; Methods?: string[] } }> }
      | undefined;
    expect(created?.Permissions?.[0]?.HTTP?.PathPrefix).toBe('/reports');
    expect(created?.Permissions?.[0]?.HTTP?.Methods).toEqual(['GET']);

  });

  test('switching a Layer 7 intention to Allow warns it will remove permissions', async ({ page, consul }) => {
    // A dedicated pair (not backed by a registered catalog service — intentions
    // don't require one) so this test can't collide with the other specs' data.
    // Needs its own service-defaults Protocol=http — Consul rejects Layer 7
    // permissions against the implicit "tcp" default (see scenarios/intentions).
    const destination = 'temp-downgrade-target';
    await consul.registerConfigEntry({ Kind: 'service-defaults', Name: destination, Protocol: 'http' });
    await consul.registerIntention({
      SourceName: 'api',
      DestinationName: destination,
      Permissions: [{ Action: 'allow', HTTP: { PathExact: '/temp' } }],
    });

    await page.goto('/ui/dc1/intentions');
    await page.getByPlaceholder('Search').fill(destination);
    await page.getByPlaceholder('Search').press('Enter');
    await page.locator('[data-test-intention-source="api"]').click();

    await page.getByRole('radio', { name: 'Allow', exact: false }).first().click();
    await page.getByRole('button', { name: 'Save' }).click();

    await expect(page.getByRole('heading', { name: 'Set intention to allow?' })).toBeVisible();
    await expect(
      page.getByText('you will remove all the Layer 7 policy permissions currently saved'),
    ).toBeVisible();

    await page.getByRole('button', { name: 'No, Cancel' }).click();
    await expect(page.getByRole('heading', { name: 'Set intention to allow?' })).toBeHidden();

    // Cancelling left the stored intention untouched.
    const seeded = await consul.intentions();
    const stillL7 = seeded.find((i) => i.SourceName === 'api' && i.DestinationName === destination);
    expect(stillL7?.Action).toBeUndefined();

  });
});
