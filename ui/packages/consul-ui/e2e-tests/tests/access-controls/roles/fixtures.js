/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

const { test: base, expect } = require('@playwright/test');

/**
 * Role API helper — creates and cleans up roles via the Consul HTTP API.
 */
class RoleApiHelper {
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
   * Creates a role via the API and tracks it for cleanup.
   * @param {object} [overrides] - Partial role body to merge with defaults.
   *   Common fields: Name, Description, Policies (array of {ID} or {Name}),
   *   ServiceIdentities, NodeIdentities.
   * @returns {Promise<object>} The created role object.
   */
  async create(overrides = {}) {
    const ts = Date.now();
    const body = {
      Name: `e2e-role-${ts}`,
      Description: `E2E test role ${ts}`,
      Policies: [],
      ServiceIdentities: [],
      NodeIdentities: [],
      ...overrides,
    };

    const response = await this.request.put(`${this.baseURL}/v1/acl/role`, {
      headers: this.headers,
      data: body,
    });

    expect(response.ok(), `Role creation failed: ${response.status()}`).toBeTruthy();
    const created = await response.json();
    this._createdIds.push(created.ID);
    return created;
  }

  /**
   * Reads a role by ID via the API.
   * @param {string} id
   * @returns {Promise<object>}
   */
  async read(id) {
    const response = await this.request.get(`${this.baseURL}/v1/acl/role/${id}`, {
      headers: this.headers,
    });
    expect(response.ok(), `Role read failed: ${response.status()}`).toBeTruthy();
    return response.json();
  }

  /**
   * Reads a role by name via the API.
   * @param {string} name
   * @returns {Promise<object>}
   */
  async readByName(name) {
    const response = await this.request.get(`${this.baseURL}/v1/acl/role/name/${name}`, {
      headers: this.headers,
    });
    expect(response.ok(), `Role read-by-name failed: ${response.status()}`).toBeTruthy();
    return response.json();
  }

  /**
   * Deletes a role by ID via the API.
   * @param {string} id
   */
  async delete(id) {
    const response = await this.request.delete(`${this.baseURL}/v1/acl/role/${id}`, {
      headers: this.headers,
    });
    // 404 is acceptable — role may have already been deleted by the test.
    if (!response.ok() && response.status() !== 404) {
      console.warn(`Warning: failed to delete role ${id} (${response.status()})`);
    }
    this._createdIds = this._createdIds.filter((i) => i !== id);
  }

  /**
   * Deletes a role by name via the API.
   * @param {string} name
   */
  async deleteByName(name) {
    const role = await this.readByName(name);
    if (role && role.ID) {
      await this.delete(role.ID);
    }
  }

  /**
   * Lists all roles via the API.
   * @returns {Promise<object[]>}
   */
  async list() {
    const response = await this.request.get(`${this.baseURL}/v1/acl/roles`, {
      headers: this.headers,
    });
    expect(response.ok(), `Role list failed: ${response.status()}`).toBeTruthy();
    return response.json();
  }

  /**
   * Deletes all roles tracked by this helper instance.
   */
  async cleanup() {
    for (const id of [...this._createdIds]) {
      await this.delete(id);
    }
  }
}

/**
 * RolesPage — page-object helper for the Consul UI roles list/detail pages.
 */
class RolesPage {
  constructor(page, baseURL) {
    this.page = page;
    this.baseURL = baseURL;
  }

  // ── Navigation ────────────────────────────────────────────────────────────

  async goto() {
    await this.page.goto(`${this.baseURL}/ui/dc1/acls/roles`, {
      waitUntil: 'domcontentloaded',
    });
  }

  async gotoCreate() {
    await this.goto();
    await this.page.getByRole('link', { name: 'Create' }).click();
  }

  async gotoRole(id) {
    await this.page.goto(`${this.baseURL}/ui/dc1/acls/roles/${id}`, {
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
   * Fills the name and optional description fields then saves the form.
   * @param {string} name
   * @param {object} [opts]
   * @param {string} [opts.description]
   * @param {string[]} [opts.policies]
   */
  async fillAndSave(name, { description, policies } = {}) {
    await this.nameInput.waitFor({ state: 'visible', timeout: 30000 });
    await this.nameInput.fill(name);
    if (description !== undefined) {
      await this.descriptionInput.fill(description);
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
   * Returns the row locator for a role matching the given name.
   * @param {string} name
   */
  roleRow(name) {
    return this.page.getByText(name).first();
  }

  /**
   * Waits for a role row to be visible in the list.
   * @param {string} name
   */
  async waitForRoleInList(name) {
    await expect(this.roleRow(name)).toBeVisible({ timeout: 30000 });
  }

  /**
   * Clicks a role row to open the detail view.
   * @param {string} name
   */
  async openRole(name) {
    await this.roleRow(name).click();
    await expect(this.page).toHaveURL(/\/roles\//, { timeout: 30000 });
  }

  /**
   * Deletes the currently viewed role and confirms the action.
   */
  async deleteCurrentRole() {
    await this.deleteButton.click();
    await this.confirmDeleteButton.click();
    await expect(this.page).toHaveURL(/\/roles$/, { timeout: 30000 });
  }

  /**
   * Extracts the role ID from the current page URL.
   * @returns {string|null}
   */
  idFromUrl() {
    const match = this.page.url().match(/\/roles\/([^/?#]+)/);
    return match ? match[1] : null;
  }
}

/**
 * Extended test object with `roleApi` and `rolesPage` fixtures.
 *
 * Usage:
 *   const { test, expect } = require('./fixtures');
 *   test('my test', async ({ page, roleApi, rolesPage }) => { ... });
 */
const test = base.extend({
  /**
   * Provides a `RoleApiHelper` instance scoped to the test.
   * Automatically cleans up any roles created during the test.
   */
  roleApi: async ({ request, baseURL }, use) => {
    const helper = new RoleApiHelper(request, baseURL);
    await use(helper);
    await helper.cleanup();
  },

  /**
   * Provides a `RolesPage` page-object instance scoped to the test.
   */
  rolesPage: async ({ page, baseURL }, use) => {
    const rolesPage = new RolesPage(page, baseURL);
    await use(rolesPage);
  },
});

module.exports = { test, expect, RoleApiHelper, RolesPage };
