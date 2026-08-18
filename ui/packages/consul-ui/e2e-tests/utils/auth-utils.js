/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

/**
 * Authentication utilities for Consul UI E2E tests
 */

// Locator helpers — sync, no count() guards needed; .or() handles DOM shape variants.

function authMenuLocator(page) {
  return page
    .locator('[data-test-auth-menu]')
    .or(page.getByRole('button', { name: 'Auth menu' }))
    .first();
}

function loginActionLocator(page) {
  return page
    .locator('[data-test-auth-menu-login]')
    .or(page.getByRole('button', { name: 'Log in' }))
    .first();
}

function logoutActionLocator(page) {
  return page
    .locator('[data-test-auth-menu-logout]')
    .or(page.getByRole('button', { name: 'Log out' }))
    .first();
}

async function openAuthMenu(page) {
  await authMenuLocator(page).click();
}

/**
 * Logs into Consul UI using a token.
 * @param {import('@playwright/test').Page} page - Playwright page object
 * @param {string} token - Consul ACL token (from env or parameter)
 * @param {string} baseURL - Base URL for the UI (optional, uses page's baseURL if not provided)
 */
async function loginWithToken(page, token = process.env.CONSUL_UI_TEST_TOKEN, baseURL = null) {
  if (!token) {
    throw new Error('CONSUL_UI_TEST_TOKEN environment variable is not set');
  }

  // Use provided baseURL or get from page context
  const url = baseURL || page.context()._options.baseURL || 'http://localhost:8500';
  await page.goto(`${url}/ui/dc1/services`, { waitUntil: 'domcontentloaded' });
  await authMenuLocator(page).waitFor({ state: 'visible', timeout: 30000 });
  await openAuthMenu(page);

  if (
    await logoutActionLocator(page)
      .isVisible()
      .catch(() => false)
  ) {
    return { authenticated: true, reason: 'already-authenticated' };
  }

  if (
    !(await loginActionLocator(page)
      .isVisible()
      .catch(() => false))
  ) {
    return { authenticated: false, reason: 'login-unavailable' };
  }

  await loginActionLocator(page).click();

  const secretInput = page
    .locator('input[name="auth[SecretID]"]')
    .or(page.getByRole('textbox', { name: 'Log in with a token' }));
  await secretInput.waitFor({ state: 'visible', timeout: 30000 });
  await secretInput.fill(token);

  await page.locator('.modal-dialog-modal').getByRole('button', { name: 'Log in' }).click();

  // Verify we transition to an authenticated menu state.
  await openAuthMenu(page);
  await logoutActionLocator(page).waitFor({ state: 'visible', timeout: 30000 });
  return { authenticated: true, reason: 'logged-in' };
}

/**
 * Checks if user is already logged in.
 * @param {import('@playwright/test').Page} page - Playwright page object
 * @param {string} baseURL - Base URL for the UI (optional, uses page's baseURL if not provided)
 * @returns {Promise<boolean>}
 */
async function isLoggedIn(page, baseURL = null) {
  try {
    // Use provided baseURL or get from page context
    const url = baseURL || page.context()._options.baseURL || 'http://localhost:8500';
    await page.goto(`${url}/ui/dc1/services`, { waitUntil: 'domcontentloaded' });
    await authMenuLocator(page).waitFor({ state: 'visible', timeout: 10000 });
    await openAuthMenu(page);
    return await logoutActionLocator(page)
      .isVisible()
      .catch(() => false);
  } catch {
    return false;
  }
}

module.exports = { loginWithToken, isLoggedIn };
