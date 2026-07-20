/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

/**
 * Enterprise (ENT) detection utilities for Consul UI E2E tests.
 *
 * Admin Partitions and other ENT-only features require Consul Enterprise.
 *
 * HOW TO GATE TESTS TO ENT-ONLY:
 *   Set the environment variable CONSUL_ENT_ONLY=true before running tests.
 *   When set, any test calling skipIfCommunityEdition() will be skipped
 *   unless the running Consul instance is Enterprise edition.
 *
 *   Currently: CONSUL_ENT_ONLY defaults to false, so tests run on CE + ENT.
 *   To flip: set CONSUL_ENT_ONLY=true in CI or your local .env file.
 */

// Flip this to 'true' to restrict ENT-only tests to Enterprise instances only.
const ENT_ONLY_MODE = process.env.CONSUL_ENT_ONLY === 'true';

/**
 * Detects if the running Consul instance is Enterprise edition by inspecting
 * the version string from /v1/agent/self (ENT versions contain "+ent").
 *
 * @param {import('@playwright/test').APIRequestContext} request
 * @param {string} baseURL
 * @param {string} [token]
 * @returns {Promise<boolean>}
 */
async function isEnterpriseConsul(request, baseURL, token) {
  try {
    const headers = token ? { 'X-Consul-Token': token } : {};
    const response = await request.get(`${baseURL}/v1/agent/self`, { headers });
    if (!response.ok()) return false;
    const data = await response.json();
    const version = (data?.Config?.Version ?? '').toLowerCase();
    return version.includes('+ent') || version.includes('-ent');
  } catch {
    return false;
  }
}

/**
 * Skips the current test when CONSUL_ENT_ONLY=true AND the running Consul
 * instance is Community Edition.
 *
 * Call this at the top of any test body that covers an ENT-only feature:
 *
 *   test('my test', async ({ page, request, baseURL }) => {
 *     await skipIfCommunityEdition(test, request, baseURL);
 *     // ...rest of test
 *   });
 *
 * @param {import('@playwright/test').TestType} test - The Playwright `test` object
 * @param {import('@playwright/test').APIRequestContext} request
 * @param {string} baseURL
 * @param {string} [token]
 */
async function skipIfCommunityEdition(
  test,
  request,
  baseURL,
  token = process.env.CONSUL_UI_TEST_TOKEN
) {
  if (!ENT_ONLY_MODE) {
    // Currently running on all editions — no skip.
    return;
  }
  const isEnt = await isEnterpriseConsul(request, baseURL, token);
  test.skip(
    !isEnt,
    'Admin Partitions is an Enterprise-only feature. ' +
      'Set CONSUL_ENT_ONLY=false to run on CE for development.'
  );
}

module.exports = { isEnterpriseConsul, skipIfCommunityEdition, ENT_ONLY_MODE };
