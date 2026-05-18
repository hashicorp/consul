/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

const base = require('@playwright/test');
const { loginWithToken } = require('../utils/auth-utils');

/**
 * Playwright fixtures for Intentions tests.
 *
 * intentionsPage  – page already logged in and on /ui/dc1/intentions
 * intentionApi    – API helpers (create/track/delete) with auto-cleanup after every test
 */
exports.test = base.test.extend({
  /**
   * intentionsPage
   * Logs in and navigates to the intentions list before handing the page to
   * the test. Replaces the boilerplate beforeEach that every test previously
   * needed.
   */
  // eslint-disable-next-line no-empty-pattern
  intentionsPage: async ({ page }, use) => {
    await loginWithToken(page);
    await page.goto('/ui/dc1/intentions');
    await use(page);
  },

  /**
   * intentionCreatePage
   * Logs in and navigates directly to the intention create form before handing
   * the page to the test. Removes the openIntentionCreate() boilerplate from
   * tests that only need to exercise the create/edit form.
   */
  // eslint-disable-next-line no-empty-pattern
  intentionCreatePage: async ({ page }, use) => {
    await loginWithToken(page);
    await page.goto('/ui/dc1/intentions/create');
    await use(page);
  },

  /**
   * intentionApi
   * Provides helpers for managing intentions via the Consul HTTP API.
   *
   *   create(source, destination, options?)
   *     Creates an intention and automatically tracks it for cleanup.
   *
   *   track(source, destination, sourceNS?, destNS?)
   *     Registers a UI-created intention so it is cleaned up after the test.
   *
   *   delete(source, destination, sourceNS?, destNS?)
   *     Deletes an intention immediately (also used internally during teardown).
   *
   * All tracked intentions are deleted after the test body finishes,
   * even when the test fails.
   */
  intentionApi: async ({ page }, use) => {
    const token = process.env.CONSUL_UI_TEST_TOKEN;
    const getBaseURL = () => page.context()._options.baseURL || 'http://localhost:4200';

    const tracked = [];

    const api = {
      async create(
        source,
        destination,
        { action = 'allow', description = '', sourceNS = 'default', destNS = 'default' } = {}
      ) {
        const response = await page.request.put(
          `${getBaseURL()}/v1/connect/intentions/exact?source=${sourceNS}/${source}&destination=${destNS}/${destination}`,
          {
            headers: { 'X-Consul-Token': token, 'Content-Type': 'application/json' },
            data: { Action: action, Description: description },
          }
        );
        console.log(
          `Created intention: ${source} -> ${destination} (status: ${response.status()})`
        );
        tracked.push({ source, destination, sourceNS, destNS });
        return { source, destination, action, description };
      },

      /**
       * Register a UI-created intention so the fixture cleans it up after the test.
       * Silently ignores falsy values so callers don't need null-guards.
       */
      track(source, destination, sourceNS = 'default', destNS = 'default') {
        if (source && destination) {
          tracked.push({ source, destination, sourceNS, destNS });
        }
      },

      async delete(source, destination, sourceNS = 'default', destNS = 'default') {
        try {
          const response = await page.request.delete(
            `${getBaseURL()}/v1/connect/intentions/exact?source=${sourceNS}/${source}&destination=${destNS}/${destination}`,
            { headers: { 'X-Consul-Token': token } }
          );
          console.log(
            `Deleted intention: ${source} -> ${destination} (status: ${response.status()})`
          );
        } catch (error) {
          console.log(
            `Note: Could not delete intention ${source} -> ${destination} - ${error.message}`
          );
        }
      },

      /**
       * Fetch the current state of an intention from the API.
       * Returns the intention JSON or null if it doesn't exist.
       */
      async get(source, destination, sourceNS = 'default', destNS = 'default') {
        try {
          const response = await page.request.get(
            `${getBaseURL()}/v1/connect/intentions/exact?source=${sourceNS}/${source}&destination=${destNS}/${destination}`,
            { headers: { 'X-Consul-Token': token } }
          );
          if (response.ok()) {
            return await response.json();
          }
          return null;
        } catch {
          return null;
        }
      },
    };

    await use(api);

    // Teardown – runs after test body, even on failure
    for (const { source, destination, sourceNS, destNS } of tracked) {
      await api.delete(source, destination, sourceNS, destNS);
    }
  },
});

exports.expect = base.expect;

// ---------------------------------------------------------------------------
// Shared page-interaction helpers
// Exported here so multiple spec files can import them without duplication.
// ---------------------------------------------------------------------------

/**
 * Find the table row that contains a specific source service name.
 * Uses CSS :has() which Playwright supports natively.
 */
exports.intentionRow = (page, source) =>
  page.locator(`tr:has([data-test-intention-source="${source}"])`);

/**
 * Click a combobox identified by its label text, select the nth option,
 * and return the selected option's trimmed text.
 */
exports.selectService = async (page, labelText, nth) => {
  const combobox = page.locator('label').filter({ hasText: labelText }).getByRole('combobox');
  await combobox.click();
  const option = page.getByRole('option').nth(nth);
  const text = await option.textContent();
  await option.click();
  return text?.trim();
};
