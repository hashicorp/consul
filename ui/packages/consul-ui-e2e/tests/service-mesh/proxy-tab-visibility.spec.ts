/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import { test, expect } from '../../fixtures/consul-scenario.ts';

test.use({ scenario: 'service-mesh' });

/**
 * Scenario: service-mesh
 *
 * Covers the tab-gating and page-level branches around the proxy tabs:
 *   - standalone  (no sidecar) -> Upstreams / Exposed Paths tabs are absent.
 *   - web-tproxy  (sidecar Proxy.Mode="transparent") -> the "Transparent proxy
 *     mode" warning renders on the Upstreams tab.
 *
 * TabNav renders each tab inside <li data-test-tab="tab_<slug>"> as an <a> link;
 * the instance route redirects fresh entries to /health-checks, so a tab is
 * reached by clicking it.
 */

test.describe('Service mesh — proxy tab visibility', () => {
  test('an instance without a sidecar shows neither proxy tab', async ({ page }) => {
    await page.goto('/ui/dc1/services/standalone/instances/node-primary/standalone');
    // Health Checks always exists, so the tab bar has rendered.
    await expect(page.locator('[data-test-tab="tab_health-checks"]')).toBeVisible();
    await expect(page.locator('[data-test-tab="tab_upstreams"]')).toHaveCount(0);
    await expect(page.locator('[data-test-tab="tab_exposed-paths"]')).toHaveCount(0);
  });

  test('transparent-mode proxy shows the transparent proxy warning', async ({ page }) => {
    await page.goto('/ui/dc1/services/web-tproxy/instances/node-primary/web-tproxy');
    await page.locator('[data-test-tab="tab_upstreams"]').click();
    await expect(page).toHaveURL(/\/upstreams$/);

    await expect(page.getByText('Transparent proxy mode')).toBeVisible();
    // The explicitly-defined upstream still renders under the warning.
    await expect(page.getByText('127.0.0.1:9391')).toBeVisible();
  });
});
