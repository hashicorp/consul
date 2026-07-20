/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

const { test, expect } = require('./fixtures');
const { isEnterpriseConsul } = require('../../utils/ent-utils');

/**
 * Namespaces - Basic Tests
 *
 * Fast, essential tests for the Namespaces feature.
 * Run on every PR.
 *
 * NOTE: Namespaces is an Enterprise-only feature.
 * Tests are tagged @ent and gated by isEnterpriseConsul().
 */

test.describe('Namespaces - Basic', { tag: '@ent' }, () => {
  test('namespaces list page loads', async ({ namespacesPage, request, baseURL }) => {
    const isEnt = await isEnterpriseConsul(request, baseURL);
    test.skip(!isEnt, 'Namespaces is an Enterprise-only feature.');

    await namespacesPage.goto();

    await expect(namespacesPage.heading).toBeVisible({ timeout: 15000 });
  });

  test('default namespace is visible in the list', async ({ namespacesPage, request, baseURL }) => {
    const isEnt = await isEnterpriseConsul(request, baseURL);
    test.skip(!isEnt, 'Namespace list requires Enterprise Consul.');

    await namespacesPage.goto();
    await expect(namespacesPage.heading).toBeVisible({ timeout: 15000 });

    await namespacesPage.waitForNamespaceInList('default');
  });

  test('namespace edit page loads from list action', async ({
    namespacesPage,
    request,
    baseURL,
  }) => {
    const isEnt = await isEnterpriseConsul(request, baseURL);
    test.skip(!isEnt, 'Namespace edit form requires Enterprise Consul.');

    await namespacesPage.goto();
    await expect(namespacesPage.heading).toBeVisible({ timeout: 15000 });

    await namespacesPage.openEditViaMoreMenu('default');

    await expect(namespacesPage.descriptionInput).toBeVisible({ timeout: 10000 });
    await expect(namespacesPage.saveButton).toBeVisible();
    await expect(namespacesPage.cancelButton).toBeVisible();
  });

  test('namespace edit page is directly navigable', async ({
    namespacesPage,
    request,
    baseURL,
  }) => {
    const isEnt = await isEnterpriseConsul(request, baseURL);
    test.skip(!isEnt, 'Namespace edit form requires Enterprise Consul.');

    await namespacesPage.gotoEdit('default');

    await expect(namespacesPage.page).toHaveURL(/\/namespaces\/default/, { timeout: 15000 });
    await expect(namespacesPage.descriptionInput).toBeVisible({ timeout: 15000 });
  });

  test('"All Namespaces" breadcrumb navigates back to list', async ({
    namespacesPage,
    request,
    baseURL,
  }) => {
    const isEnt = await isEnterpriseConsul(request, baseURL);
    test.skip(!isEnt, 'Namespace edit form requires Enterprise Consul.');

    await namespacesPage.gotoEdit('default');
    await expect(namespacesPage.page).toHaveURL(/\/namespaces\/default/, { timeout: 15000 });

    await namespacesPage.allNamespacesBreadcrumb.click();

    await expect(namespacesPage.page).toHaveURL(/\/namespaces$/, { timeout: 15000 });
    await expect(namespacesPage.heading).toBeVisible({ timeout: 10000 });
  });

  test('default namespace has no delete option', async ({ namespacesPage, request, baseURL }) => {
    const isEnt = await isEnterpriseConsul(request, baseURL);
    test.skip(!isEnt, 'Namespace list requires Enterprise Consul.');

    await namespacesPage.goto();
    await expect(namespacesPage.heading).toBeVisible({ timeout: 15000 });
    await namespacesPage.waitForNamespaceInList('default');

    // Open More menu for default and verify Delete is absent
    await namespacesPage.moreButtonForNamespace('default').click();
    await expect(namespacesPage.page.getByRole('menuitem', { name: 'Delete' })).toHaveCount(0, {
      timeout: 5000,
    });
  });
});
