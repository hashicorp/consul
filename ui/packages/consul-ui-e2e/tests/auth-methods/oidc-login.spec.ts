/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import { test, expect } from '../../fixtures/consul-scenario.ts';

test.use({ scenario: 'auth-methods' });

/**
 * Scenario: auth-methods
 *
 * Drives a REAL OIDC login through consul-ui's popup flow against the Dex
 * IdP this scenario boots (see scenarios/auth-methods/README.md) — not a
 * mock of the flow, the actual thing: a real popup, a real third-party
 * login form, a real authorization-code exchange, a real Consul ACL token.
 *
 * Two scenario-specific requirements, both explained in
 * scenarios/auth-methods/README.md:
 *
 *   1. Must run with `E2E_BASE_URL=http://127.0.0.1:4200` (not the usual
 *      `localhost`) — Dex's registered redirect_uri is 127.0.0.1, and OIDC
 *      redirect-URI matching is exact.
 *   2. The login menu needs ACLs on, and the login dialog's SSO tab needs
 *      SSO on. A built Enterprise binary reports both to the UI itself; the
 *      Ember dev server defaults both off and reads them from the
 *      `CONSUL_ACLS_ENABLE` / `CONSUL_SSO_ENABLE` cookies instead (see
 *      consul-ui's `app/utils/get-environment.js`), so set them before the
 *      page loads. Production builds ignore these cookies.
 */
test.describe('OIDC login', { tag: '@ent' }, () => {
  test.beforeEach(async ({ context, baseURL }) => {
    if (!baseURL?.includes('127.0.0.1')) {
      throw new Error(
        `OIDC login needs a 127.0.0.1 base URL (got baseURL="${baseURL}") — see scenarios/auth-methods/README.md`,
      );
    }
    await context.addCookies([
      { name: 'CONSUL_ACLS_ENABLE', value: '1', url: baseURL },
      { name: 'CONSUL_SSO_ENABLE', value: '1', url: baseURL },
    ]);
  });

  test('logs in through a real Dex popup and mints an ACL token', async ({ page, consul }) => {
    await page.goto('/ui/dc1/services');

    await page.locator('[data-test-auth-menu]').click();
    await page.locator('[data-test-auth-menu-login]').click();
    await expect(page.getByRole('heading', { name: 'Log in to Consul' })).toBeVisible();

    // Token/SSO tabs aren't semantic (no role="tab") — see auth-methods-detail.spec.ts.
    await page.locator('[data-test-tab="tab_sso"]').click();

    // The SSO tab lands on an Admin Partition picker first (`OidcSelect`'s
    // own idle state) — "default" is already filled in, just confirm it —
    // *then* the provider list ("Continue with dex-oidc") renders.
    await page.getByRole('button', { name: 'Choose provider' }).click();

    // Scoped to `.oidc-select` (OidcSelect's own root element): the dialog's
    // OWN "Continue without logging in" dismiss button also matches a loose
    // /Continue with/ match (it's a literal substring of "Continue without").
    const ssoSelect = page.locator('.oidc-select');
    const popupPromise = page.waitForEvent('popup');
    await ssoSelect.getByRole('button', { name: /Continue with/ }).click();
    const popup = await popupPromise;
    await popup.waitForLoadState();

    // Dex's static-password connector login form (see scenarios/auth-methods/README.md).
    await popup.locator('#login').fill('demo@example.com');
    await popup.locator('#password').fill('consul-demo-pw');
    await popup.locator('#submit-login').click();

    // Dex redirects the popup to our redirect_uri (.../ui/oidc/callback —
    // consul-ui's rootURL is "/ui/"); torii polls the popup's same-origin
    // location and closes it itself once it sees the code.
    await popup.waitForEvent('close', { timeout: 15_000 });

    // A successful login closes the dialog itself (consul-ui's
    // Consul::Token::Selector#reauthorize calls modal.close() before
    // propagating the token) — then the user menu shows "Log out".
    await expect(page.getByRole('heading', { name: 'Log in to Consul' })).not.toBeVisible();
    await page.locator('[data-test-auth-menu]').click();
    await expect(page.locator('[data-test-auth-menu-logout]')).toBeVisible();

    // And a real Consul token exists, minted by dex-oidc. `acl:read` is
    // never covered by `default_policy = "allow"` (see `loginAs`'s doc
    // comment) — the `consul` fixture has no token by default, so use the
    // management one to list.
    consul.useToken('1234');
    const tokens = await consul.tokens();
    const minted = tokens.find((t) => t.AuthMethod === 'dex-oidc');
    expect(
      minted,
      `expected a token minted by dex-oidc — found AuthMethods: ${tokens.map((t) => t.AuthMethod).join(', ')}`,
    ).toBeTruthy();
  });
});
