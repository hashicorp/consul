/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

const { test, expect } = require('../../fixtures/services');

/**
 * Services - Basic Tests
 *
 * Fast, essential tests for Services feature
 * Run on every PR
 */

test.describe('Services - Basic Tests', () => {
  test('verify services and topology', async ({ servicesPage }) => {
    // 1. Navigate to service page
    await servicesPage.gotoList();

    // Verify all services are present. (consul is there by default)
    const expectedServices = [
      'consul',
      'product-db',
      'product-api',
      'payments',
      'public-api',
      'frontend',
    ];
    for (const s of expectedServices) {
      await expect(
        servicesPage.page.getByRole('link', { name: s, exact: true }).first()
      ).toBeVisible();
    }

    const verifyServiceFlow = async (serviceName, expectedUpstreams = []) => {
      await servicesPage.gotoList();
      await servicesPage.navigateToService(serviceName);

      // Verify it lands on the topology tab by default
      await expect(servicesPage.page).toHaveURL(/.*\/topology$/);
      await expect(servicesPage.page.getByRole('heading', { name: serviceName })).toBeVisible();

      // Verify upstreams in topology
      for (const upstream of expectedUpstreams) {
        await expect(
          servicesPage.page
            .locator('#upstream-container .topology-metrics-card')
            .filter({ hasText: upstream })
            .first()
        ).toBeVisible();
      }

      await servicesPage.clickTab('Instances');

      // Navigate to service instances from instances tab
      const firstInstanceLink = servicesPage.page
        .locator('.consul-service-instance-list li')
        .nth(1)
        .locator('a')
        .first();
      await expect(firstInstanceLink).toBeVisible();
      await firstInstanceLink.click();

      // Verify we landed on instance details
      await expect(
        servicesPage.page.locator('.title').filter({ hasText: serviceName }).first()
      ).toBeVisible();

      if (expectedUpstreams.length > 0) {
        await servicesPage.clickTab('Upstreams');
        for (const upstream of expectedUpstreams) {
          const upstreamLink = servicesPage.page
            .locator('.consul-upstream-instance-list li')
            .filter({ hasText: upstream })
            .first();
          await expect(upstreamLink).toBeVisible();
        }
      }
    };

    await verifyServiceFlow('frontend', ['public-api']);
    await verifyServiceFlow('public-api', ['payments', 'product-api']);
    await verifyServiceFlow('product-api', ['product-db']);
    await verifyServiceFlow('payments', []);
    await verifyServiceFlow('product-db', []);
  });
});
