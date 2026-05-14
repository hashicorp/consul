/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

const { test: base, expect } = require('@playwright/test');

/**
 * Token API helper — creates and cleans up tokens via the Consul HTTP API.
 */
class TokenApiHelper {
  constructor(request, baseURL) {
    this.request = request;
    this.baseURL = baseURL;
    this.token = process.env.CONSUL_UI_TEST_TOKEN;
    this._createdIds = [];
  }

  get headers() {
    return { 'X-Consul-Token': this.token };
  }

  /**
   * Creates a token via the API and tracks it for cleanup.
   * @param {object} [overrides] - Partial token body to merge with defaults.
   * @returns {Promise<object>} The created token object.
   */
  async create(overrides = {}) {
    const body = {
      Description: `E2E test token ${Date.now()}`,
      Policies: [],
      Roles: [],
      Local: false,
      ...overrides,
    };

    const response = await this.request.put(`${this.baseURL}/v1/acl/token`, {
      headers: this.headers,
      data: body,
    });

    expect(response.ok(), `Token creation failed: ${response.status()}`).toBeTruthy();
    const created = await response.json();
    this._createdIds.push(created.AccessorID);
    return created;
  }

  /**
   * Reads a token by AccessorID via the API.
   * @param {string} accessorId
   * @returns {Promise<object>}
   */
  async read(accessorId) {
    const response = await this.request.get(`${this.baseURL}/v1/acl/token/${accessorId}`, {
      headers: this.headers,
    });
    expect(response.ok(), `Token read failed: ${response.status()}`).toBeTruthy();
    return response.json();
  }

  /**
   * Deletes a token by AccessorID via the API.
   * @param {string} accessorId
   */
  async delete(accessorId) {
    const response = await this.request.delete(`${this.baseURL}/v1/acl/token/${accessorId}`, {
      headers: this.headers,
    });
    // 404 is acceptable — token may have already been deleted by the test.
    if (!response.ok() && response.status() !== 404) {
      console.warn(`Warning: failed to delete token ${accessorId} (${response.status()})`);
    }
    this._createdIds = this._createdIds.filter((id) => id !== accessorId);
  }

  /**
   * Lists all tokens via the API.
   * @returns {Promise<object[]>}
   */
  async list() {
    const response = await this.request.get(`${this.baseURL}/v1/acl/tokens`, {
      headers: this.headers,
    });
    expect(response.ok(), `Token list failed: ${response.status()}`).toBeTruthy();
    return response.json();
  }

  /**
   * Reads a token by exact description via list endpoint.
   * @param {string} description
   * @returns {Promise<object|null>}
   */
  async readByDescription(description) {
    const tokens = await this.list();
    return tokens.find((t) => (t.Description || '') === description) || null;
  }

  /**
   * Deletes a token by exact description via list endpoint.
   * @param {string} description
   */
  async deleteByDescription(description) {
    const token = await this.readByDescription(description);
    if (token && token.AccessorID) {
      await this.delete(token.AccessorID);
    }
  }

  /**
   * Deletes all tokens tracked by this helper instance.
   */
  async cleanup() {
    for (const id of [...this._createdIds]) {
      await this.delete(id);
    }
  }
}

/**
 * TokensPage — page-object helper for the Consul UI tokens list/detail pages.
 */

class TokensPage {
  constructor(page, baseURL) {
    this.page = page;
    this.baseURL = baseURL;
  }

  // ── Navigation ────────────────────────────────────────────────────────────

  async goto() {
    await this.page.goto(`${this.baseURL}/ui/dc1/acls/tokens`, { waitUntil: 'domcontentloaded' });
  }

  async gotoCreate() {
    await this.goto();
    await this.page.getByRole('link', { name: 'Create' }).click();
  }

  async gotoToken(accessorId) {
    await this.page.goto(`${this.baseURL}/ui/dc1/acls/tokens/${accessorId}`, {
      waitUntil: 'domcontentloaded',
    });
  }

  // ── Form helpers ──────────────────────────────────────────────────────────

  get descriptionInput() {
    return this.page.getByRole('textbox', { name: 'Description (Optional)' });
  }

  get saveButton() {
    return this.page.getByRole('button', { name: 'Save' });
  }

  get deleteButton() {
    return this.page.getByRole('button', { name: 'Delete' });
  }

  get confirmDeleteButton() {
    return this.page.getByRole('button', { name: 'Confirm Delete' });
  }

  async selectFromSuperSelect(label, optionText) {
    const field = await this.page.getByText(label);
    await field.waitFor({ state: 'visible', timeout: 30000 });
    await field.click();
    await field.fill(optionText);

    const option = this.page
      .getByRole('option', { name: optionText, exact: true })
      .or(this.page.getByText(optionText, { exact: true }))
      .first();
    await option.waitFor({ state: 'visible', timeout: 30000 });
    await option.click();
  }

  /**
   * Fills the description field (and roles/policies) and saves the token form.
   */
  async fillAndSave(description, { roles, policies } = {}) {
    await this.descriptionInput.waitFor({ state: 'visible', timeout: 30000 });
    await this.descriptionInput.fill(description);
    if (roles) {
      for (const roleText of roles) {
        await this.selectFromSuperSelect('Search for role', roleText);
      }
    }
    if (policies) {
      for (const policyText of policies) {
        await this.selectFromSuperSelect('Search for policy', policyText);
      }
    }
    await this.saveButton.click();
  }

  async editAndSave({ description } = {}) {
    if (description !== undefined) {
      await this.descriptionInput.fill(description);
    }
    await this.saveButton.click();
  }

  /**
   * Returns the row locator for a token matching the given description.
   * @param {string} description
   */
  tokenRow(description) {
    return this.page
      .locator('[data-test-description]')
      .filter({ hasText: description })
      .first()
      .locator('xpath=ancestor::*[contains(@class,"list-collection") or self::li][1]');
  }

  /**
   * Waits for a token row to be visible in the list.
   * @param {string} description
   */
  async waitForTokenInList(description) {
    await expect(this.tokenRow(description)).toBeVisible({ timeout: 30000 });
  }

  /**
   * Clicks on a token row to open the detail view.
   * @param {string} description
   */
  async openToken(description) {
    const row = this.tokenRow(description);
    await expect(row).toBeVisible({ timeout: 30000 });
    const tokenLink = row.locator('[data-test-token]').first();
    await tokenLink.click();
    await expect(this.page).toHaveURL(/\/tokens\//, { timeout: 30000 });
  }

  async useTokenFromList(description) {
    const rowContainer = this.tokenRow(description);
    await expect(rowContainer).toBeVisible({ timeout: 30000 });

    const useAction = rowContainer.locator('[data-test-use-action]').first();
    if (!(await useAction.isVisible().catch(() => false))) {
      const moreButton = rowContainer.getByRole('button', { name: /more/i }).first();
      if (await moreButton.isVisible().catch(() => false)) {
        await moreButton.click();
      }
    }

    if (await useAction.isVisible().catch(() => false)) {
      await useAction.click();
      await this.page.getByRole('button', { name: /^Use$/ }).first().click();
      return;
    }

    // fallback to open then click use
    await this.openToken(description);
    await this.page.locator('[data-test-use]').click();
    await this.page.locator('[data-test-confirm-use]').click();
  }

  /**
   * Deletes the currently viewed token and confirms the action.
   */
  async deleteCurrentToken() {
    await this.deleteButton.click();
    await this.confirmDeleteButton.click();
    await expect(this.page).toHaveURL(/\/tokens$/, { timeout: 30000 });
  }

  /**
   * Extracts the AccessorID from the current page URL.
   * @returns {string|null}
   */
  accessorIdFromUrl() {
    const match = this.page.url().match(/\/tokens\/([^/?#]+)/);
    return match ? match[1] : null;
  }
}

/**
 * Extended test object with `tokenApi` and `tokensPage` fixtures.
 *
 *
 **/
const test = base.extend({
  /**
   * Provides a `TokenApiHelper` instance scoped to the test.
   * Automatically cleans up any tokens created during the test.
   */
  tokenApi: async ({ request, baseURL }, use) => {
    const helper = new TokenApiHelper(request, baseURL);
    await use(helper);
    await helper.cleanup();
  },

  /**
   * Provides a `TokensPage` page-object instance scoped to the test.
   */
  tokensPage: async ({ page, baseURL }, use) => {
    const tokensPage = new TokensPage(page, baseURL);
    await use(tokensPage);
  },
});

module.exports = {
  test,
  expect,
  TokenApiHelper,
  TokensPage,
};
