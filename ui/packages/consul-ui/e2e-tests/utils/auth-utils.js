/**
 * Authentication utilities for Consul UI E2E tests
 */

/**
 * Logs into Consul UI using a token
 * @param {import('@playwright/test').Page} page - Playwright page object
 * @param {string} token - Consul ACL token (from env or parameter)
 */
export async function loginWithToken(page, token = process.env.CONSUL_UI_TEST_TOKEN) {
  if (!token) {
    throw new Error('CONSUL_UI_TEST_TOKEN environment variable is not set');
  }

  // Navigate to the UI
  await page.goto('http://localhost:4200/ui/dc1/services');

  // Click on "Tokens" link in the navigation
  await page.getByRole('link', { name: 'Tokens' }).click();

  // Click "Log in" button
  await page.getByRole('button', { name: 'Log in' }).click();

  // Fill in the token
  await page.getByRole('textbox', { name: 'Log in with a token' }).click();
  await page.getByRole('textbox', { name: 'Log in with a token' }).fill(token);

  // Submit the login form
  await page.getByLabel('Log in to Consul').getByRole('button', { name: 'Log in' }).click();

  // Wait for navigation to complete (login successful)
  await page.waitForURL('**/ui/dc1/**');
}

/**
 * Checks if user is already logged in
 * @param {import('@playwright/test').Page} page - Playwright page object
 * @returns {Promise<boolean>} - True if logged in, false otherwise
 */
export async function isLoggedIn(page) {
  try {
    // Check if we can access a protected page without being redirected
    await page.goto('http://localhost:4200/ui/dc1/services', { waitUntil: 'networkidle' });
    
    // If "Log in" button is visible, user is not logged in
    const loginButton = page.getByRole('button', { name: 'Log in' });
    return !(await loginButton.isVisible({ timeout: 2000 }));
  } catch {
    return false;
  }
}

// Made with Bob
