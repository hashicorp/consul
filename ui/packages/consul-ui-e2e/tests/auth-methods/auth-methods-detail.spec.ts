/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import { test, expect, loginAs } from '../../fixtures/consul-scenario.ts';

test.use({ scenario: 'auth-methods' });

/**
 * Scenario: auth-methods
 * Routes: /ui/dc1/acls/auth-methods/:name (General info + Binding rules tabs)
 *
 * `/ui/dc1/acls/*` is ACL-gated — see `loginAs`'s doc comment.
 *
 * Tabs are selected via TabNav's `data-test-tab` hook
 * (`tab_<slugified-label>`). Discovery URL and Host render as masked HDS
 * inputs, so their values are read from the named textbox.
 */
test.describe('Auth method detail', { tag: '@ent' }, () => {
  test.beforeEach(async ({ page, consul }) => {
    await loginAs(page, consul, '1234'); // management token — see scenarios/auth-methods/README.md
  });

  test('dex-oidc: type badge, discovery URL, and its role-type binding rule', async ({ page }) => {
    await page.goto('/ui/dc1/acls/auth-methods/dex-oidc');
    await expect(page.locator('[data-test-type="oidc"]').first()).toBeVisible();
    await expect(page.getByRole('textbox', { name: 'Discovery URL' })).toHaveValue('http://127.0.0.1:5556/dex');

    await page.locator('[data-test-tab="tab_binding-rules"]').click();
    await expect(page.getByRole('heading', { name: 'oidc-demo-role' })).toBeVisible();
    await expect(page.getByText('role', { exact: true })).toBeVisible();
    await expect(page.locator('code', { hasText: 'value.email == "demo@example.com"' })).toBeVisible();
  });

  test('dex-jwt: jwt type badge and the same discovery URL, no UI login path', async ({ page }) => {
    await page.goto('/ui/dc1/acls/auth-methods/dex-jwt');
    await expect(page.locator('[data-test-type="jwt"]').first()).toBeVisible();
    await expect(page.getByRole('textbox', { name: 'Discovery URL' })).toHaveValue('http://127.0.0.1:5556/dex');
  });

  test('k8s-demo: kubernetes type badge, Host, and its service-type binding rule', async ({ page }) => {
    await page.goto('/ui/dc1/acls/auth-methods/k8s-demo');
    await expect(page.locator('[data-test-type="kubernetes"]').first()).toBeVisible();
    await expect(page.getByRole('textbox', { name: 'Host' })).toHaveValue(/^https:\/\/k8s-demo\.example\.com/);

    await page.locator('[data-test-tab="tab_binding-rules"]').click();
    await expect(page.getByRole('heading', { name: '${serviceaccount.name}' })).toBeVisible();
    await expect(page.getByText('service', { exact: true })).toBeVisible();
    await expect(page.locator('code', { hasText: 'serviceaccount.namespace==default' })).toBeVisible();
  });
});
