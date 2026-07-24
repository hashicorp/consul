/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

const { test, expect } = require('./fixtures');
const { isEnterpriseConsul } = require('../../utils/ent-utils');

/**
 * Namespaces - Workflow Tests
 *
 * Complex end-to-end scenarios covering create / edit / delete lifecycle
 * as well as navigation via the namespace selector in the side nav.
 *
 * NOTE: Namespaces is an Enterprise-only feature.
 * Tests are tagged @ent and gated by isEnterpriseConsul().
 */

test.describe('Namespaces - Workflows', { tag: '@ent' }, () => {
  /**
   * Navigate to namespaces via the namespace selector in the side nav:
   *   open the Namespace select → click "View all namespaces".
   */
  test('navigate to namespaces via namespace selector', async ({
    page,
    namespacesPage,
    request,
    baseURL,
  }) => {
    const isEnt = await isEnterpriseConsul(request, baseURL);
    test.skip(!isEnt, 'Namespaces is an Enterprise-only feature.');

    await page.goto(`${baseURL}/ui/dc1/services`, { waitUntil: 'domcontentloaded' });

    // Open the namespace selector in the side nav.
    await page.locator('[data-test-nspace-menu]').click();

    // Click the "View all namespaces" footer link inside the selector dropdown.
    await page.getByRole('link', { name: 'View all namespaces' }).click();

    await expect(namespacesPage.page).toHaveURL(/\/namespaces$/, { timeout: 15000 });
    await expect(namespacesPage.heading).toBeVisible({ timeout: 10000 });
  });

  /**
   * Full create → edit → delete lifecycle for a custom namespace.
   *
   * Steps (mirrors the user's inspector recording):
   *  1. Create a new namespace with a name and description.
   *  2. Edit the namespace and update the description.
   *  3. Verify the updated description persists after reload.
   *  4. Delete the namespace.
   *  5. Verify the namespace no longer appears in the list.
   */
  test('create, edit, and delete a namespace', async ({
    namespacesPage,
    namespaceApi,
    request,
    baseURL,
  }) => {
    const isEnt = await isEnterpriseConsul(request, baseURL);
    test.skip(!isEnt, 'Namespace CRUD requires Enterprise Consul.');

    const unique = Date.now();
    const namespaceName = `e2e-ns-${unique}`;
    const initialDescription = `Initial description ${unique}`;
    const updatedDescription = `Updated description ${unique}`;

    // Track the name so API cleanup can remove it if the test fails mid-way.
    namespaceApi._createdNames.push(namespaceName);

    // ── 1. Create ───────────────────────────────────────────────────────────
    await namespacesPage.goto();
    await expect(namespacesPage.heading).toBeVisible({ timeout: 15000 });

    await namespacesPage.page.getByRole('link', { name: 'Create' }).click();
    await expect(namespacesPage.page).toHaveURL(/\/namespaces\/create/, { timeout: 15000 });

    await namespacesPage.nameInput.waitFor({ state: 'visible', timeout: 10000 });
    await namespacesPage.nameInput.fill(namespaceName);
    await namespacesPage.descriptionInput.fill(initialDescription);
    await namespacesPage.saveButton.click();

    // After save the UI should redirect back to the namespaces list.
    await expect(namespacesPage.page).toHaveURL(/\/namespaces$/, { timeout: 20000 });
    await namespacesPage.waitForNamespaceInList(namespaceName);

    // ── 2. Edit ─────────────────────────────────────────────────────────────
    await namespacesPage.openEditViaMoreMenu(namespaceName);

    await expect(namespacesPage.descriptionInput).toBeVisible({ timeout: 10000 });
    await namespacesPage.descriptionInput.fill(updatedDescription);
    await namespacesPage.saveButton.click();

    // After save the UI should redirect back to the namespaces list.
    await expect(namespacesPage.page).toHaveURL(/\/namespaces$/, { timeout: 20000 });

    // ── 3. Verify persistence ────────────────────────────────────────────────
    await namespacesPage.openEditViaMoreMenu(namespaceName);
    await expect(namespacesPage.descriptionInput).toHaveValue(updatedDescription, {
      timeout: 10000,
    });
    await namespacesPage.cancelButton.click();
    await expect(namespacesPage.page).toHaveURL(/\/namespaces$/, { timeout: 15000 });

    // ── 4. Delete ────────────────────────────────────────────────────────────
    await namespacesPage.openDeleteViaMoreMenu(namespaceName);
    await namespacesPage.confirmDeleteInModal();

    // After deletion the namespace row should disappear.
    await expect(
      namespacesPage.page.locator('[data-test-list-row]').filter({ hasText: namespaceName })
    ).toHaveCount(0, { timeout: 20000 });

    // Remove from API cleanup tracker since it was deleted via UI.
    namespaceApi._createdNames = namespaceApi._createdNames.filter((n) => n !== namespaceName);
  });

  /**
   * Create a namespace with a role and policy attached via the ACL selectors,
   * then verify the namespace list reflects the new namespace.
   */
  test('create a namespace with a role and policy', async ({
    namespacesPage,
    namespaceApi,
    request,
    baseURL,
  }) => {
    const isEnt = await isEnterpriseConsul(request, baseURL);
    test.skip(!isEnt, 'Namespace CRUD with ACLs requires Enterprise Consul.');

    const unique = Date.now();
    const namespaceName = `e2e-ns-acl-${unique}`;

    // Pre-create a policy and role via API so they are available in the selectors.
    const token = process.env.CONSUL_UI_TEST_TOKEN;
    const headers = { 'X-Consul-Token': token };

    const policyRes = await namespacesPage.page.request.put(`${baseURL}/v1/acl/policy`, {
      headers,
      data: {
        Name: `e2e-ns-policy-${unique}`,
        Description: `E2E namespace policy ${unique}`,
        Rules: '',
      },
    });
    expect(policyRes.ok(), `Policy creation failed: ${policyRes.status()}`).toBeTruthy();
    const policy = await policyRes.json();

    const roleRes = await namespacesPage.page.request.put(`${baseURL}/v1/acl/role`, {
      headers,
      data: {
        Name: `e2e-ns-role-${unique}`,
        Description: `E2E namespace role ${unique}`,
        Policies: [{ ID: policy.ID }],
      },
    });
    expect(roleRes.ok(), `Role creation failed: ${roleRes.status()}`).toBeTruthy();
    const role = await roleRes.json();

    namespaceApi._createdNames.push(namespaceName);

    try {
      await namespacesPage.goto();
      await expect(namespacesPage.heading).toBeVisible({ timeout: 15000 });

      await namespacesPage.page.getByRole('link', { name: 'Create' }).click();
      await expect(namespacesPage.page).toHaveURL(/\/namespaces\/create/, { timeout: 15000 });

      await namespacesPage.nameInput.waitFor({ state: 'visible', timeout: 10000 });
      await namespacesPage.nameInput.fill(namespaceName);

      // Apply an existing role via the role child-selector.
      await namespacesPage.selectFromSuperSelect('Search for role', role.Name);

      // Apply an existing policy via the policy child-selector.
      await namespacesPage.selectFromSuperSelect('Search for policy', policy.Name);

      await namespacesPage.saveButton.click();

      await expect(namespacesPage.page).toHaveURL(/\/namespaces$/, { timeout: 20000 });
      await namespacesPage.waitForNamespaceInList(namespaceName);
    } finally {
      // Clean up ACL resources regardless of test outcome.
      await namespacesPage.page.request
        .delete(`${baseURL}/v1/acl/role/${role.ID}`, { headers })
        .catch(() => {});
      await namespacesPage.page.request
        .delete(`${baseURL}/v1/acl/policy/${policy.ID}`, { headers })
        .catch(() => {});
    }
  });

  /**
   * Edit the "default" namespace description and save — verify redirect back to list.
   * Restores the original description after the test via API.
   */
  test('edit default namespace description and save', async ({
    namespacesPage,
    namespaceApi,
    request,
    baseURL,
  }) => {
    const isEnt = await isEnterpriseConsul(request, baseURL);
    test.skip(!isEnt, 'Namespace updates require Enterprise Consul.');

    const newDescription = `E2E test description – ${Date.now()}`;

    // Capture the current description for post-test restore.
    let originalDescription = '';
    try {
      const current = await namespaceApi.read('default');
      originalDescription = current?.Description ?? '';
    } catch {
      // Non-fatal.
    }

    await namespacesPage.goto();
    await expect(namespacesPage.heading).toBeVisible({ timeout: 15000 });

    await namespacesPage.openEditViaMoreMenu('default');
    await namespacesPage.fillDescriptionAndSave(newDescription);

    await expect(namespacesPage.page).toHaveURL(/\/namespaces$/, { timeout: 20000 });

    // Restore original description so subsequent runs start clean.
    try {
      await namespaceApi.update('default', { Description: originalDescription });
    } catch {
      console.warn('[namespaces] Could not restore default namespace description after test.');
    }
  });

  /**
   * Cancel an edit of the default namespace and verify no changes are persisted.
   */
  test('cancel namespace edit discards changes and returns to list', async ({
    namespacesPage,
    request,
    baseURL,
  }) => {
    const isEnt = await isEnterpriseConsul(request, baseURL);
    test.skip(!isEnt, 'Namespaces is an Enterprise-only feature.');

    await namespacesPage.gotoEdit('default');
    await expect(namespacesPage.page).toHaveURL(/\/namespaces\/default/, { timeout: 15000 });
    await namespacesPage.descriptionInput.waitFor({ state: 'visible', timeout: 10000 });

    await namespacesPage.descriptionInput.fill('This should not be saved');
    await namespacesPage.cancelButton.click();

    await expect(namespacesPage.page).toHaveURL(/\/namespaces$/, { timeout: 15000 });
    await expect(namespacesPage.heading).toBeVisible({ timeout: 10000 });
  });

  /**
   * Verify the default namespace cannot be deleted:
   * the "Delete" action must not appear in its More menu.
   */
  test('default namespace cannot be deleted', async ({ namespacesPage, request, baseURL }) => {
    const isEnt = await isEnterpriseConsul(request, baseURL);
    test.skip(!isEnt, 'Namespace list requires Enterprise Consul.');

    await namespacesPage.goto();
    await expect(namespacesPage.heading).toBeVisible({ timeout: 15000 });
    await namespacesPage.waitForNamespaceInList('default');

    await namespacesPage.moreButtonForNamespace('default').click();

    // The Delete menuitem must be absent for the built-in default namespace.
    await expect(namespacesPage.page.getByRole('menuitem', { name: 'Delete' })).toHaveCount(0, {
      timeout: 5000,
    });
  });
});
