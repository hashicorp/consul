/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

const { test: base, expect } = require('@playwright/test');

/**
 * Policy API helper — creates and cleans up policies via the Consul HTTP API.
 */
class PolicyApiHelper {
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
   * Creates a policy via the API and tracks it for cleanup.
   * @param {object} [overrides] - Partial policy body to merge with defaults.
   * @returns {Promise<object>} The created policy object.
   */
  async create(overrides = {}) {
    const ts = Date.now();
    const body = {
      Name: `e2e-policy-${ts}`,
      Description: `E2E test policy ${ts}`,
      Rules: '',
      Datacenters: [],
      ...overrides,
    };

    const response = await this.request.put(`${this.baseURL}/v1/acl/policy`, {
      headers: this.headers,
      data: body,
    });

    expect(response.ok(), `Policy creation failed: ${response.status()}`).toBeTruthy();
    const created = await response.json();
    this._createdIds.push(created.ID);
    return created;
  }

  /**
   * Reads a policy by ID via the API.
   * @param {string} id
   * @returns {Promise<object>}
   */
  async read(id) {
    const response = await this.request.get(`${this.baseURL}/v1/acl/policy/${id}`, {
      headers: this.headers,
    });
    expect(response.ok(), `Policy read failed: ${response.status()}`).toBeTruthy();
    return response.json();
  }

  /**
   * Reads a policy by name via the API.
   * @param {string} name
   * @returns {Promise<object>}
   */
  async readByName(name) {
    const response = await this.request.get(`${this.baseURL}/v1/acl/policy/name/${name}`, {
      headers: this.headers,
    });
    expect(response.ok(), `Policy read-by-name failed: ${response.status()}`).toBeTruthy();
    return response.json();
  }

  /**
   * Deletes a policy by ID via the API.
   * @param {string} id
   */
  async delete(id) {
    const response = await this.request.delete(`${this.baseURL}/v1/acl/policy/${id}`, {
      headers: this.headers,
    });
    // 404 is acceptable — policy may have already been deleted by the test.
    if (!response.ok() && response.status() !== 404) {
      console.warn(`Warning: failed to delete policy ${id} (${response.status()})`);
    }
    this._createdIds = this._createdIds.filter((i) => i !== id);
  }

  /**
   * Deletes a policy by name via the API.
   * @param {string} name
   */
  async deleteByName(name) {
    const policy = await this.readByName(name);
    if (policy && policy.ID) {
      await this.delete(policy.ID);
    }
  }

  /**
   * Lists all policies via the API.
   * @returns {Promise<object[]>}
   */
  async list() {
    const response = await this.request.get(`${this.baseURL}/v1/acl/policies`, {
      headers: this.headers,
    });
    expect(response.ok(), `Policy list failed: ${response.status()}`).toBeTruthy();
    return response.json();
  }

  /**
   * Deletes all policies tracked by this helper instance.
   */
  async cleanup() {
    for (const id of [...this._createdIds]) {
      await this.delete(id);
    }
  }
}

/**
 * PoliciesPage — page-object helper for the Consul UI policies list/detail pages.
 */
class PoliciesPage {
  constructor(page, baseURL) {
    this.page = page;
    this.baseURL = baseURL;
  }

  // ── Navigation ────────────────────────────────────────────────────────────

  async goto() {
    await this.page.goto(`${this.baseURL}/ui/dc1/acls/policies`, {
      waitUntil: 'domcontentloaded',
    });
  }

  async gotoCreate() {
    await this.goto();
    await this.page.getByRole('link', { name: 'Create' }).click();
  }

  async gotoPolicy(id) {
    await this.page.goto(`${this.baseURL}/ui/dc1/acls/policies/${id}`, {
      waitUntil: 'domcontentloaded',
    });
  }

  // ── Form helpers ──────────────────────────────────────────────────────────

  get nameInput() {
    return this.page.getByRole('textbox', { name: 'Name' });
  }

  get descriptionInput() {
    return this.page.getByRole('textbox', { name: 'Description' });
  }

  get rulesEditor() {
    // The rules editor is a CodeMirror / textarea; fall back to textarea if needed.
    return this.page
      .locator('[data-test-policy-rules]')
      .or(this.page.locator('textarea[name="rules"]'))
      .first();
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

  /**
   * Fills the name (and optional description/rules) fields and saves the form.
   * @param {string} name
   * @param {object} [opts]
   * @param {string} [opts.description]
   * @param {string} [opts.rules]
   */
  async fillAndSave(name, { description, rules } = {}) {
    await this.nameInput.waitFor({ state: 'visible', timeout: 30000 });
    await this.nameInput.fill(name);
    if (description !== undefined) {
      await this.descriptionInput.fill(description);
    }
    if (rules !== undefined) {
      await this.rulesEditor.fill(rules);
    }
    await this.saveButton.click();
  }

  async editAndSave({ description, rules } = {}) {
    if (description !== undefined) {
      await this.descriptionInput.fill(description);
    }
    if (rules !== undefined) {
      await this.rulesEditor.fill(rules);
    }
    await this.saveButton.click();
  }

  /**
   * Returns the row locator for a policy matching the given name.
   * @param {string} name
   */
  policyRow(name) {
    return this.page.getByText(name).first();
  }

  /**
   * Waits for a policy row to be visible in the list.
   * @param {string} name
   */
  async waitForPolicyInList(name) {
    await expect(this.policyRow(name)).toBeVisible({ timeout: 30000 });
  }

  /**
   * Clicks a policy row to open the detail view.
   * @param {string} name
   */
  async openPolicy(name) {
    await this.policyRow(name).click();
    await expect(this.page).toHaveURL(/\/policies\//, { timeout: 30000 });
  }

  /**
   * Deletes the currently viewed policy and confirms the action.
   */
  async deleteCurrentPolicy() {
    await this.deleteButton.click();
    await this.confirmDeleteButton.click();
    await expect(this.page).toHaveURL(/\/policies$/, { timeout: 30000 });
  }

  /**
   * Extracts the policy ID from the current page URL.
   * @returns {string|null}
   */
  idFromUrl() {
    const match = this.page.url().match(/\/policies\/([^/?#]+)/);
    return match ? match[1] : null;
  }
}

/**
 * Extended test object with `policyApi` and `policiesPage` fixtures.
 *
 * Usage:
 *   const { test, expect } = require('./fixtures');
 *   test('my test', async ({ page, policyApi, policiesPage }) => { ... });
 */
const test = base.extend({
  /**
   * Provides a `PolicyApiHelper` instance scoped to the test.
   * Automatically cleans up any policies created during the test.
   */
  policyApi: async ({ request, baseURL }, use) => {
    const helper = new PolicyApiHelper(request, baseURL);
    await use(helper);
    await helper.cleanup();
  },

  /**
   * Provides a `PoliciesPage` page-object instance scoped to the test.
   */
  policiesPage: async ({ page, baseURL }, use) => {
    const policiesPage = new PoliciesPage(page, baseURL);
    await use(policiesPage);
  },
});

module.exports = { test, expect, PolicyApiHelper, PoliciesPage };
