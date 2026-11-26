/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import { test, expect } from '../../fixtures/consul-scenario.ts';
import type { Page } from '@playwright/test';

test.use({ scenario: 'intentions' });

/**
 * Scenario: intentions
 * Routes: /ui/dc1/intentions, /ui/dc1/intentions/create, /ui/dc1/intentions/:id
 *
 * Drives the create form and the edit/delete flow end to end. Each test uses
 * its own disposable Source/Destination pair (created and cleaned up via the
 * `consul` fixture, or deleted through the UI as the test itself) rather than
 * the scenario's shared seeded rows — so these specs stay independent of
 * `intentions-list.spec.ts` / `permissions.spec.ts` regardless of run order,
 * per docs/adding-a-test.md.
 */

/**
 * The table row whose destination matches `destination`. `[data-test-intention]`
 * sits on a `<span>` inside the Source cell only — not the row — so anchor on
 * the ARIA `row` instead of that span to reach sibling cells (action, actions
 * menu, ...).
 */
function rowFor(page: Page, destination: string) {
  return page.getByRole('row').filter({ has: page.locator(`[data-test-intention-destination="${destination}"]`) });
}

/**
 * Pick `name` in the Source/Destination SuperSelectWithCreate at `wrapper`
 * (`[data-test-source-element]` / `[data-test-destination-element]`).
 *
 * The trigger and its search input are both `role="combobox"` once expanded,
 * and the page also has a real `<input>` (Description) that also satisfies
 * `getByRole('textbox')` — so `.last()` on the scoped combobox pair is what
 * actually reaches the search box, not a bare `getByRole('textbox')`.
 */
async function pickService(page: Page, wrapper: string, name: string, { create = false } = {}) {
  const el = page.locator(wrapper);
  await el.getByRole('combobox').click();
  await el.getByRole('combobox').last().fill(name);
  if (create) {
    await page.getByText(`Use a Consul Service called '${name}'`).click();
  } else {
    await page.getByRole('option', { name, exact: true }).click();
  }
}

test.describe('Intentions create / edit / delete', () => {
  // Remove what these tests create even when one fails part-way, so the seeded
  // intentions the list spec counts stay exact. Deleting a missing intention is
  // a no-op in Consul.
  test.afterEach(async ({ consul }) => {
    for (const destination of ['crud-create-target', 'crud-edit-target', 'crud-delete-target']) {
      await consul.deleteIntention({ SourceName: 'web', DestinationName: destination });
    }
  });

  test('creates a new Allow intention via the form', async ({ page, consul }) => {
    const destination = 'crud-create-target';
    await page.goto('/ui/dc1/intentions/create');

    await pickService(page, '[data-test-source-element]', 'web');
    // Not a registered service — exercises "type a service that doesn't exist
    // yet" (SuperSelectWithCreate's "Use a Consul Service called '...'").
    await pickService(page, '[data-test-destination-element]', destination, { create: true });

    // "Allow" is the form's default Action — just save.
    await page.getByRole('button', { name: 'Save' }).click();

    await expect(page).toHaveURL(/\/ui\/dc1\/intentions$/);
    await expect(page.getByText('Your intention has been saved.')).toBeVisible();
    await expect(rowFor(page, destination).locator('[data-test-intention-action="allow"]')).toBeVisible();

    const seeded = await consul.intentions();
    expect(seeded.some((i) => i.SourceName === 'web' && i.DestinationName === destination)).toBe(true);

  });

  test('edit page disables the Source/Destination pickers', async ({ page, consul }) => {
    const destination = 'crud-edit-target';
    await consul.registerIntention({ SourceName: 'web', DestinationName: destination, Action: 'allow' });

    await page.goto('/ui/dc1/intentions');
    await page.getByPlaceholder('Search').fill(destination);
    await page.getByPlaceholder('Search').press('Enter');
    await rowFor(page, destination).locator('[data-test-intention-source]').click();

    await expect(page).toHaveURL(/\/ui\/dc1\/intentions\/.+/);
    await expect(page.locator('[data-test-source-element]').getByRole('combobox')).toBeDisabled();
    await expect(page.locator('[data-test-destination-element]').getByRole('combobox')).toBeDisabled();
    // Description / Action / Delete remain live.
    await expect(page.getByRole('button', { name: 'Delete' })).toBeEnabled();

  });

  test('deletes an intention with confirmation', async ({ page, consul }) => {
    const destination = 'crud-delete-target';
    await consul.registerIntention({ SourceName: 'web', DestinationName: destination, Action: 'allow' });

    await page.goto('/ui/dc1/intentions');
    await page.getByPlaceholder('Search').fill(destination);
    await page.getByPlaceholder('Search').press('Enter');
    await expect(rowFor(page, destination)).toBeVisible();

    await rowFor(page, destination).locator('[data-test-actions-menu]').click();
    await page.locator('[data-test-delete]').click();
    await page.locator("#confirm-modal [data-test-id='confirm-action']").click();

    await expect(page.getByText('Your intention has been deleted.')).toBeVisible();
    await expect(rowFor(page, destination)).toHaveCount(0);

    const seeded = await consul.intentions();
    expect(seeded.some((i) => i.SourceName === 'web' && i.DestinationName === destination)).toBe(false);
  });
});
