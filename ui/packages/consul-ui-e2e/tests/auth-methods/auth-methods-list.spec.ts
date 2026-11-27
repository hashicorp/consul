/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import { test, expect, loginAs } from '../../fixtures/consul-scenario.ts';

test.use({ scenario: 'auth-methods' });

/**
 * Scenario: auth-methods
 * Route: /ui/dc1/acls/auth-methods
 *
 * `/ui/dc1/acls/*` is ACL-gated (`acl:read` is never covered by
 * `default_policy = "allow"`) — see `loginAs`'s doc comment.
 *
 * The list renders regardless of the `CONSUL_SSO_ENABLE` cookie (see
 * oidc-login.spec.ts) — the OIDC type badge itself isn't gated by that flag,
 * only the login dialog's SSO tab and the list's "OIDC" type-filter option
 * are. So these specs don't set it.
 */
test.describe('Auth methods list', { tag: '@ent' }, () => {
  test.beforeEach(async ({ page, consul }) => {
    await loginAs(page, consul, '1234'); // management token — see scenarios/auth-methods/README.md
    await page.goto('/ui/dc1/acls/auth-methods');
    await expect(page.locator('[data-test-auth-method]').first()).toBeVisible();
  });

  test('renders the seeded auth methods with their type badges', async ({ page }) => {
    const rows: Array<[name: string, type: string]> = [
      ['dex-oidc', 'oidc'],
      ['dex-jwt', 'jwt'],
      ['k8s-demo', 'kubernetes'],
    ];
    for (const [name, type] of rows) {
      const row = page.locator('[data-test-auth-method]', { hasText: name }).locator('xpath=ancestor::tr');
      await expect(row).toBeVisible();
      await expect(row.locator(`[data-test-type="${type}"]`)).toBeVisible();
    }
  });

  test('sorts by name ascending by default', async ({ page }) => {
    const names = await page.locator('[data-test-name]').allTextContents();
    const seeded = names.filter((n) => ['dex-oidc', 'dex-jwt', 'k8s-demo'].includes(n));
    expect(seeded).toEqual(['dex-jwt', 'dex-oidc', 'k8s-demo']);
  });

  test('searches by name', async ({ page }) => {
    // exact: true — the page also has global nav "Search partitions"/"Search
    // namespaces" pickers with the same placeholder prefix. `.fill()` alone
    // sets the value but doesn't trigger the search — this box filters on
    // keystroke events, not on a plain `input` event, so type it out and
    // press Enter.
    const search = page.getByRole('searchbox', { name: 'Search', exact: true });
    await search.pressSequentially('k8s');
    await search.press('Enter');
    await expect(page.locator('[data-test-auth-method]', { hasText: 'k8s-demo' })).toBeVisible();
    await expect(page.locator('[data-test-auth-method]', { hasText: 'dex-oidc' })).toHaveCount(0);
    await expect(page.locator('[data-test-auth-method]', { hasText: 'dex-jwt' })).toHaveCount(0);
  });
});
