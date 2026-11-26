/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import { test, expect } from '../../fixtures/consul-scenario.ts';

test.use({ scenario: 'service-mesh' });

/**
 * Scenario: service-mesh
 * Route: /ui/dc1/services/web/instances/node-primary/web
 *
 * The Upstreams and Exposed Paths tabs only appear when the instance has a
 * sidecar proxy (Kind=connect-proxy pointing back at it). Asserts both tabs
 * render and carry the seeded data, including the Address branch (LocalBindPort)
 * and every per-field Exposed Paths row.
 *
 * The instance route redirects any fresh entry to /health-checks, so a tab is
 * reached by clicking it (a client-side transition) rather than deep-linking.
 * TabNav renders HDS tabs (role="tab") carrying data-test-tab="tab_<slug>".
 */
const INSTANCE = '/ui/dc1/services/web/instances/node-primary/web';

test.describe('Service mesh — web instance', () => {
  test('sidecar proxy is registered', async ({ consul }) => {
    const sidecar = await consul.agentService('web-sidecar-proxy');
    expect(sidecar.Kind).toBe('connect-proxy');
  });

  test('both proxy tabs are present on the instance', async ({ page }) => {
    await page.goto(INSTANCE);
    await expect(page.locator('[data-test-tab="tab_upstreams"]')).toBeVisible();
    await expect(page.locator('[data-test-tab="tab_exposed-paths"]')).toBeVisible();
  });

  test('Upstreams tab lists db and cache with bind addresses', async ({ page }) => {
    await page.goto(INSTANCE);
    await page.locator('[data-test-tab="tab_upstreams"]').click();
    await expect(page).toHaveURL(/\/upstreams$/);

    await expect(page.getByText('db', { exact: false }).first()).toBeVisible();
    await expect(page.getByText('cache', { exact: false }).first()).toBeVisible();
    // Address branch: LocalBindAddress defaults to 127.0.0.1 + LocalBindPort.
    await expect(page.getByText('127.0.0.1:9191')).toBeVisible();
    await expect(page.getByText('127.0.0.1:9192')).toBeVisible();
  });

  test('Exposed Paths tab renders every per-field row', async ({ page }) => {
    await page.goto(INSTANCE);
    await page.locator('[data-test-tab="tab_exposed-paths"]').click();
    await expect(page).toHaveURL(/\/exposed-paths$/);

    // Path row. exact:true targets the Path <dd> and not the combined-address
    // span (which reads "<addr>:<port>/healthz").
    await expect(page.getByText('/healthz', { exact: true })).toBeVisible();
    await expect(page.getByText('/metrics', { exact: true })).toBeVisible();
    // Listener Port row ("listening on :<ListenerPort>").
    await expect(page.getByText('listening on :21500')).toBeVisible();
    await expect(page.getByText('listening on :21501')).toBeVisible();
    // Local Path Port row ("local port :<LocalPathPort>").
    await expect(page.getByText('local port :8080').first()).toBeVisible();
    // Protocol row.
    await expect(page.getByText('http', { exact: true }).first()).toBeVisible();
  });
});
