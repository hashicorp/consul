/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

const { test, expect } = require('./fixtures');

/**
 * Admin Partitions - Basic Tests
 *
 * Fast, essential tests for the Admin Partitions feature.
 * Run on every PR.
 *
 * NOTE: Admin Partitions is an Enterprise-only feature.
 * These tests currently run on CE + ENT for development purposes.
 * To restrict to ENT only: set CONSUL_ENT_ONLY=true in the environment.
 */

test.describe('Admin Partitions - Basic', () => {
  test('admin partitions list page loads', async ({ partitionsPage, skipEnt, request, baseURL }) => {
    await skipEnt(request, baseURL);

    await partitionsPage.goto();

    await expect(partitionsPage.heading).toBeVisible({ timeout: 15000 });
  });

  test('default partition is visible in the list', async ({
    partitionsPage,
    skipEnt,
    request,
    baseURL,
  }) => {
    await skipEnt(request, baseURL);

    await partitionsPage.goto();
    await expect(partitionsPage.heading).toBeVisible({ timeout: 15000 });

    // The built-in "default" partition must always be present.
    await expect(partitionsPage.partitionRow('default')).toBeVisible({ timeout: 15000 });
    await expect(partitionsPage.partitionRow('default')).toHaveText('default');
  });

  test('partition edit page loads from list action', async ({
    partitionsPage,
    skipEnt,
    request,
    baseURL,
  }) => {
    await skipEnt(request, baseURL);

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
    skipEnt,
    request,
    baseURL,
  }) => {
    await skipEnt(request, baseURL);

    await partitionsPage.gotoEdit('default');

    await expect(partitionsPage.page).toHaveURL(/\/partitions\/default/, { timeout: 15000 });
    await expect(partitionsPage.descriptionInput).toBeVisible({ timeout: 15000 });
  });

  test('"All Admin Partitions" breadcrumb navigates back to list', async ({
    partitionsPage,
    skipEnt,
    request,
    baseURL,
  }) => {
    await skipEnt(request, baseURL);

    await partitionsPage.gotoEdit('default');
    await expect(partitionsPage.page).toHaveURL(/\/partitions\/default/, { timeout: 15000 });

    await partitionsPage.allPartitionsBreadcrumb.click();

    await expect(partitionsPage.page).toHaveURL(/\/partitions$/, { timeout: 15000 });
    await expect(partitionsPage.heading).toBeVisible({ timeout: 10000 });
  });
});
