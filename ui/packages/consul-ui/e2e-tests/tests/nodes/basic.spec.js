/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

const { test, expect } = require('../../fixtures/nodes');

test.describe('Nodes - Basic Tests', () => {
  test('should navigate to nodes page and verify tabs on node instance', async ({
    nodesFixture,
  }) => {
    const { page } = nodesFixture;

    // 1. Navigate to nodes page, verify there are 8 nodes present.
    await nodesFixture.gotoNodesPage();
    await expect(page.locator('.consul-node-list ul > li:has(.header)')).toHaveCount(8);

    // 2. Navigate to frontend-0 node instance.
    await nodesFixture.gotoNodeInstance('frontend-0');

    // Verify it opens default to Health Checks tab
    await expect(page).toHaveURL(/.*\/health-checks$/);

    // Verify it has more than 1 health check
    const healthChecksList = page.locator('.consul-health-check-list li.health-check-output');
    await expect(healthChecksList.first()).toBeVisible();
    const healthCheckCards = await healthChecksList.count();
    expect(healthCheckCards).toBeGreaterThan(0);

    // 3. Navigate to Service instances tab, verify it has 1 instance present.
    await nodesFixture.openServiceInstancesTab();
    await expect(page).toHaveURL(/.*\/service-instances$/);
    await expect(page.locator('.consul-service-instance-list ul > li:has(.header)')).toHaveCount(1);

    // 4. Navigate to Lock Sessions, verify there are no Lock sessions and it shows a default page.
    await nodesFixture.openLockSessionsTab();
    await expect(page).toHaveURL(/.*\/lock-sessions$/);
    await expect(page.getByText('Documentation on Lock Sessions')).toBeVisible();
    await expect(page.locator('.consul-lock-session-list ul > li:has(.header)')).toHaveCount(0);

    // 5. Navigate to Metadata, verify it has 2 metadata rows.
    await nodesFixture.openMetadataTab();
    await expect(page).toHaveURL(/.*\/metadata$/);
    await expect(
      page.locator('.consul-metadata-list tbody tr').filter({ has: page.locator('td') })
    ).toHaveCount(2);
  });
});
