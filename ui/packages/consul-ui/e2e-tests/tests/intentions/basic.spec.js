/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

const { test, expect, intentionRow, selectService } = require('../../fixtures/intentions');

/**
 * Intentions - Basic Tests
 *
 * Fast, essential tests for Intentions feature
 * Run on every PR
 *
 * Uses the intentions fixtures:
 *   intentionsPage  – page already logged in and on /ui/dc1/intentions
 *   intentionApi    – API helpers with automatic post-test cleanup
 */

test.describe('Intentions - Basic Tests', () => {
  test('should navigate to Intentions page', async ({ intentionsPage }) => {
    await expect(intentionsPage).toHaveURL(/\/ui\/dc1\/intentions/);
    await expect(intentionsPage.getByRole('link', { name: 'Create' })).toBeVisible();
  });

  test('should validate required fields on create form', async ({ intentionCreatePage }) => {
    await expect(intentionCreatePage).toHaveURL(/\/ui\/dc1\/intentions\/create/);
    await expect(
      intentionCreatePage.locator('label').filter({ hasText: 'Source Service *' }).first()
    ).toBeVisible();
    await expect(
      intentionCreatePage.locator('label').filter({ hasText: 'Destination Service *' }).first()
    ).toBeVisible();
    await expect(intentionCreatePage.getByRole('button', { name: 'Save' })).toBeVisible();
    await expect(intentionCreatePage.getByRole('button', { name: 'Cancel' })).toBeVisible();
  });

  test('should create an intention with description', async ({
    intentionCreatePage,
    intentionApi,
  }) => {
    await intentionCreatePage
      .getByRole('textbox', { name: 'Description (Optional)' })
      .fill('Intention 1');
    const source = await selectService(intentionCreatePage, 'Source Service', 1); // nth(1) skips wildcard
    const dest = await selectService(intentionCreatePage, 'Destination Service', 2);

    await intentionCreatePage.getByRole('button', { name: 'Save' }).click();
    await expect(intentionCreatePage).toHaveURL(/\/ui\/dc1\/intentions$/);

    intentionApi.track(source, dest);
  });

  test('should create an intention with deny action', async ({
    intentionCreatePage,
    intentionApi,
  }) => {
    await intentionCreatePage
      .getByRole('textbox', { name: 'Description (Optional)' })
      .fill('Deny intention test');
    const source = await selectService(intentionCreatePage, 'Source Service', 1);
    const dest = await selectService(intentionCreatePage, 'Destination Service', 2);

    await intentionCreatePage.getByRole('radio', { name: 'Deny' }).click();
    await intentionCreatePage.getByRole('button', { name: 'Save' }).click();
    await expect(intentionCreatePage).toHaveURL(/\/ui\/dc1\/intentions$/);

    intentionApi.track(source, dest);
  });

  test('should create an intention without description', async ({
    intentionCreatePage,
    intentionApi,
  }) => {
    const source = await selectService(intentionCreatePage, 'Source Service', 1);
    const dest = await selectService(intentionCreatePage, 'Destination Service', 2);

    await intentionCreatePage.getByRole('button', { name: 'Save' }).click();
    await expect(intentionCreatePage).toHaveURL(/\/ui\/dc1\/intentions$/);

    intentionApi.track(source, dest);
  });

  test('should create intention with wildcard source', async ({
    intentionCreatePage,
    intentionApi,
  }) => {
    await intentionCreatePage
      .getByRole('textbox', { name: 'Description (Optional)' })
      .fill('Wildcard source test');

    const sourceCombobox = intentionCreatePage
      .locator('label')
      .filter({ hasText: 'Source Service' })
      .getByRole('combobox');
    await sourceCombobox.click();
    await intentionCreatePage.getByRole('option', { name: '* (All Services)' }).click();

    const dest = await selectService(intentionCreatePage, 'Destination Service', 1);

    await intentionCreatePage.getByRole('button', { name: 'Save' }).click();
    await expect(intentionCreatePage).toHaveURL(/\/ui\/dc1\/intentions$/);

    intentionApi.track('*', dest);
  });

  test('should create intention with wildcard destination', async ({
    intentionCreatePage,
    intentionApi,
  }) => {
    await intentionCreatePage
      .getByRole('textbox', { name: 'Description (Optional)' })
      .fill('Wildcard destination test');

    const source = await selectService(intentionCreatePage, 'Source Service', 1);

    const destCombobox = intentionCreatePage
      .locator('label')
      .filter({ hasText: 'Destination Service' })
      .getByRole('combobox');
    await destCombobox.click();
    await intentionCreatePage.getByRole('option', { name: '* (All Services)' }).click();

    await intentionCreatePage.getByRole('button', { name: 'Save' }).click();
    await expect(intentionCreatePage).toHaveURL(/\/ui\/dc1\/intentions$/);

    intentionApi.track(source, '*');
  });

  test('should cancel intention creation', async ({ intentionCreatePage }) => {
    await intentionCreatePage
      .getByRole('textbox', { name: 'Description (Optional)' })
      .fill('Test description');
    await intentionCreatePage.getByRole('button', { name: 'Cancel' }).click();

    await expect(intentionCreatePage).toHaveURL(/\/ui\/dc1\/intentions$/);
  });
});

test.describe('Intentions - View and List', () => {
  test('should display an API-created intention in the list', async ({
    intentionsPage,
    intentionApi,
  }) => {
    await intentionApi.create('e2e-list-src', 'e2e-list-dest', { action: 'allow' });
    await intentionsPage.goto('/ui/dc1/intentions', { waitUntil: 'networkidle' });

    await expect(
      intentionsPage.locator('[data-test-intention-source="e2e-list-src"]')
    ).toBeVisible();
    await expect(
      intentionsPage.locator('[data-test-intention-destination="e2e-list-dest"]')
    ).toBeVisible();
    await expect(
      intentionRow(intentionsPage, 'e2e-list-src').locator('[data-test-intention-action]')
    ).toHaveAttribute('data-test-intention-action', 'allow');
  });

  test('should navigate to edit page when clicking an intention', async ({
    intentionsPage,
    intentionApi,
  }) => {
    await intentionApi.create('e2e-nav-src', 'e2e-nav-dest', { action: 'allow' });
    await intentionsPage.goto('/ui/dc1/intentions', { waitUntil: 'networkidle' });

    await intentionsPage.locator('[data-test-intention-source="e2e-nav-src"]').click();
    // URL is /ui/dc1/intentions/{partition}:{ns}:{source}:{partition}:{ns}:{dest} (no /edit suffix)
    await expect(intentionsPage).toHaveURL(/\/ui\/dc1\/intentions\/[^/]+$/);
    // edit page should show source and destination
    await expect(intentionsPage.getByText('e2e-nav-src')).toBeVisible();
    await expect(intentionsPage.getByText('e2e-nav-dest')).toBeVisible();
  });
});

// ---------------------------------------------------------------------------

test.describe('Intentions - Edit', () => {
  test('should edit an intention from allow to deny', async ({ intentionsPage, intentionApi }) => {
    await intentionApi.create('e2e-allow-src', 'e2e-allow-dest', { action: 'allow' });
    await intentionsPage.goto('/ui/dc1/intentions', { waitUntil: 'networkidle' });

    // click the source link to open the edit form
    await intentionsPage.locator('[data-test-intention-source="e2e-allow-src"]').click();
    await expect(intentionsPage).toHaveURL(/\/ui\/dc1\/intentions\/[^/]+$/);

    // Use the RadioCard's value-{intent} class (unique to the main form fieldset)
    await intentionsPage.locator('.value-deny input[name="Action"]').click();
    await intentionsPage.getByRole('button', { name: 'Save' }).click();
    await expect(intentionsPage).toHaveURL(/\/ui\/dc1\/intentions$/);

    // verify the list reflects the new action
    await expect(
      intentionRow(intentionsPage, 'e2e-allow-src').locator('[data-test-intention-action]')
    ).toHaveAttribute('data-test-intention-action', 'deny');

    // also verify via the API
    const updated = await intentionApi.get('e2e-allow-src', 'e2e-allow-dest');
    expect(updated?.Action).toBe('deny');
  });

  test('should edit an intention from deny to allow', async ({ intentionsPage, intentionApi }) => {
    await intentionApi.create('e2e-deny-src', 'e2e-deny-dest', { action: 'deny' });
    await intentionsPage.goto('/ui/dc1/intentions', { waitUntil: 'networkidle' });

    await intentionsPage.locator('[data-test-intention-source="e2e-deny-src"]').click();
    await expect(intentionsPage).toHaveURL(/\/ui\/dc1\/intentions\/[^/]+$/);

    // The RadioCard adds class="value-{intent}" which is unique to the main form fieldset
    await intentionsPage.locator('.value-allow input[name="Action"]').click();
    await intentionsPage.getByRole('button', { name: 'Save' }).click();
    await expect(intentionsPage).toHaveURL(/\/ui\/dc1\/intentions$/);

    await expect(
      intentionRow(intentionsPage, 'e2e-deny-src').locator('[data-test-intention-action]')
    ).toHaveAttribute('data-test-intention-action', 'allow');

    const updated = await intentionApi.get('e2e-deny-src', 'e2e-deny-dest');
    expect(updated?.Action).toBe('allow');
  });

  test('should cancel editing and leave the intention unchanged', async ({
    intentionsPage,
    intentionApi,
  }) => {
    await intentionApi.create('e2e-cancel-src', 'e2e-cancel-dest', { action: 'allow' });
    await intentionsPage.goto('/ui/dc1/intentions', { waitUntil: 'networkidle' });

    await intentionsPage.locator('[data-test-intention-source="e2e-cancel-src"]').click();
    await expect(intentionsPage).toHaveURL(/\/ui\/dc1\/intentions\/[^/]+$/);

    // change the radio but then cancel (use value-{intent} class to avoid ambiguity)
    await intentionsPage.locator('.value-deny input[name="Action"]').click();
    await intentionsPage.getByRole('button', { name: 'Cancel' }).click();
    await expect(intentionsPage).toHaveURL(/\/ui\/dc1\/intentions$/);

    // action should remain allow
    await expect(
      intentionRow(intentionsPage, 'e2e-cancel-src').locator('[data-test-intention-action]')
    ).toHaveAttribute('data-test-intention-action', 'allow');
  });
});

// ---------------------------------------------------------------------------

test.describe('Intentions - Delete', () => {
  test('should delete an intention via the row menu', async ({ intentionsPage, intentionApi }) => {
    await intentionApi.create('e2e-del-src', 'e2e-del-dest', { action: 'allow' });
    await intentionsPage.goto('/ui/dc1/intentions', { waitUntil: 'networkidle' });

    // open the "More" popup menu for this specific row
    await intentionRow(intentionsPage, 'e2e-del-src').getByText('More').click();

    // click the Delete button inside the row's dangerous menu item
    // (scope to row avoids strict-mode violation — all rows render [data-test-delete] in the DOM)
    await intentionRow(intentionsPage, 'e2e-del-src')
      .locator('[data-test-delete] [role="menuitem"]')
      .click();

    // the MenuItem teleports an Hds::Modal to <body> — wait for it, then confirm
    await expect(intentionsPage.locator('#confirm-modal')).toBeVisible({ timeout: 5000 });
    await intentionsPage.locator('[data-test-id="confirm-action"]').click();

    // row should disappear from the list
    await expect(
      intentionsPage.locator('[data-test-intention-source="e2e-del-src"]')
    ).not.toBeVisible({ timeout: 10000 });

    // verify via API that it is truly gone
    const deleted = await intentionApi.get('e2e-del-src', 'e2e-del-dest');
    expect(deleted).toBeNull();
  });
});
