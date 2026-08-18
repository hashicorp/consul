/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

const base = require('@playwright/test');
const { loginWithToken } = require('../utils/auth-utils');

/**
 * Playwright fixtures for Nodes tests.
 */
exports.test = base.test.extend({
  nodesFixture: async ({ page }, use) => {
    await loginWithToken(page);

    const fixture = {
      page,
      async gotoNodesPage() {
        await page.goto('/ui/dc1/nodes');
      },
      async gotoNodeInstance(nodeName) {
        await page.getByRole('link', { name: nodeName, exact: true }).click();
      },
      async openHealthChecksTab() {
        await page.getByRole('link', { name: /Health Checks/i }).click();
      },
      async openServiceInstancesTab() {
        await page.getByRole('link', { name: /Service Instances/i }).click();
      },
      async openLockSessionsTab() {
        await page.getByRole('link', { name: /Lock Sessions/i }).click();
      },
      async openMetadataTab() {
        await page.getByRole('link', { name: /Metadata/i }).click();
      },
    };

    await use(fixture);
  },
});

exports.expect = base.expect;
