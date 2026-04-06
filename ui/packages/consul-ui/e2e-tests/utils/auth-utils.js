/**
 * Authentication utilities for Consul UI E2E tests
 */

async function openAuthMenu(page) {
  if ((await page.locator('[data-test-auth-menu]').count()) > 0) {
    await page.locator('[data-test-auth-menu]').click();
    return;
  }
  await page.getByRole('button', { name: 'Auth menu' }).click();
}

async function loginActionLocator(page) {
  if ((await page.locator('[data-test-auth-menu-login]').count()) > 0) {
    return page.locator('[data-test-auth-menu-login]');
  }
  return page.getByRole('button', { name: 'Log in' }).first();
}

async function logoutActionLocator(page) {
  if ((await page.locator('[data-test-auth-menu-logout]').count()) > 0) {
    return page.locator('[data-test-auth-menu-logout]');
  }
  return page.getByRole('button', { name: 'Log out' }).first();
}

/**
 * Logs into Consul UI using a token.
 * Uses data-test selectors from the app to avoid brittle role/name lookups.
 * @param {import('@playwright/test').Page} page - Playwright page object
 * @param {string} token - Consul ACL token (from env or parameter)
 */
async function loginWithToken(page, token = process.env.CONSUL_UI_TEST_TOKEN) {
  if (!token) {
    throw new Error('CONSUL_UI_TEST_TOKEN environment variable is not set');
  }

  await page.goto('http://localhost:4200/ui/dc1/services', { waitUntil: 'domcontentloaded' });
  await page.getByRole('button', { name: 'Auth menu' }).waitFor({ state: 'visible', timeout: 30000 });

  // Open auth menu and detect current auth state.
  await openAuthMenu(page);

  const loginAction = await loginActionLocator(page);
  const logoutAction = await logoutActionLocator(page);

  if (await logoutAction.isVisible().catch(() => false)) {
    return { authenticated: true, reason: 'already-authenticated' };
  }

  if (!(await loginAction.isVisible().catch(() => false))) {
    return { authenticated: false, reason: 'login-unavailable' };
  }

  await loginAction.click();

  const secretInput = page
    .locator('input[name="auth[SecretID]"]')
    .or(page.getByRole('textbox', { name: 'Log in with a token' }));
  await secretInput.waitFor({ state: 'visible', timeout: 30000 });
  await secretInput.fill(token);

  await page.getByRole('button', { name: 'Log in' }).last().click();

  // Verify we transition to an authenticated menu state.
  await openAuthMenu(page);
  await (await logoutActionLocator(page)).waitFor({ state: 'visible', timeout: 30000 });
  return { authenticated: true, reason: 'logged-in' };
}

/**
 * Checks if user is already logged in.
 * @param {import('@playwright/test').Page} page - Playwright page object
 * @returns {Promise<boolean>} - True if logged in, false otherwise
 */
async function isLoggedIn(page) {
  try {
    await page.goto('http://localhost:4200/ui/dc1/services', { waitUntil: 'domcontentloaded' });
    await page.getByRole('button', { name: 'Auth menu' }).waitFor({ state: 'visible', timeout: 10000 });
    await openAuthMenu(page);
    return await (await logoutActionLocator(page)).isVisible().catch(() => false);
  } catch {
    return false;
  }
}

module.exports = { loginWithToken, isLoggedIn };
