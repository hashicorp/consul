/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

const { test, expect } = require('./fixtures');
const { isEnterpriseConsul } = require('../../utils/ent-utils');

/**
 * Admin Partitions - Basic Tests
 *
 * Fast, essential tests for the Admin Partitions feature.
 * Run on every PR.
 *
 * NOTE: Admin Partitions is an Enterprise-only feature.
 * These tests are skipped automatically on Community Edition.
 */

test.describe('Admin Partitions - Basic', { tag: '@ent' }, () => {
  test('admin partitions list page loads', async ({ partitionsPage, request, baseURL }) => {
    const isEnt = await isEnterpriseConsul(request, baseURL);
    test.skip(!isEnt, 'Admin Partitions is an Enterprise-only feature.');

    await partitionsPage.goto();

    await expect(partitionsPage.heading).toBeVisible({ timeout: 15000 });
  });

  test('default partition is visible in the list', async ({ partitionsPage, request, baseURL }) => {
    const isEnt = await isEnterpriseConsul(request, baseURL);
    test.skip(!isEnt, 'Partition list rows require Enterprise Consul.');

    await partitionsPage.goto();
    await expect(partitionsPage.heading).toBeVisible({ timeout: 15000 });

    // The built-in "default" partition must always be present.
    await expect(partitionsPage.partitionRow('default')).toBeVisible({ timeout: 15000 });
    await expect(partitionsPage.partitionRow('default')).toHaveText('default');
  });

  test('partition edit page loads from list action', async ({
    partitionsPage,
    request,
    baseURL,
  }) => {
    const isEnt = await isEnterpriseConsul(request, baseURL);
    test.skip(!isEnt, 'Partition edit form requires Enterprise Consul.');

    await partitionsPage.goto();
    await expect(partitionsPage.heading).toBeVisible({ timeout: 15000 });

    await partitionsPage.openEditViaMoreMenu('default');

    // Edit form must contain the description input and action buttons.
    await expect(partitionsPage.descriptionInput).toBeVisible({ timeout: 10000 });
    await expect(partitionsPage.saveButton).toBeVisible();
    await expect(partitionsPage.cancelButton).toBeVisible();
  });

  test('partition edit page is directly navigable', async ({
    partitionsPage,
    request,
    baseURL,
  }) => {
    const isEnt = await isEnterpriseConsul(request, baseURL);
    test.skip(!isEnt, 'Partition edit form requires Enterprise Consul.');

    await partitionsPage.gotoEdit('default');

    await expect(partitionsPage.page).toHaveURL(/\/partitions\/default/, { timeout: 15000 });
    await expect(partitionsPage.descriptionInput).toBeVisible({ timeout: 15000 });
  });

  test('"All Admin Partitions" breadcrumb navigates back to list', async ({
    partitionsPage,
    request,
    baseURL,
  }) => {
    const isEnt = await isEnterpriseConsul(request, baseURL);
    test.skip(!isEnt, 'Partition edit form requires Enterprise Consul.');

    await partitionsPage.gotoEdit('default');
    await expect(partitionsPage.page).toHaveURL(/\/partitions\/default/, { timeout: 15000 });

    await partitionsPage.allPartitionsBreadcrumb.click();

    await expect(partitionsPage.page).toHaveURL(/\/partitions$/, { timeout: 15000 });
    await expect(partitionsPage.heading).toBeVisible({ timeout: 10000 });
  });
});
