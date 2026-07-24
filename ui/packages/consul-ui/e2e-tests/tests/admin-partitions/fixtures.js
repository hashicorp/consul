/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

const { test: base, expect } = require('@playwright/test');
const { skipIfCommunityEdition } = require('../../utils/ent-utils');

/**
 * PartitionApiHelper — creates and manages partitions via the Consul HTTP API.
 *
 * NOTE: The built-in "default" partition cannot be created or deleted via API.
 * This helper is used to create/cleanup custom partitions in workflow tests.
 */
class PartitionApiHelper {
  constructor(request, baseURL) {
    this.request = request;
    this.baseURL = baseURL;
    this.token = process.env.CONSUL_UI_TEST_TOKEN;
    this._createdNames = [];
  }

  get headers() {
    return { 'X-Consul-Token': this.token };
  }

  /**
   * Creates a partition via the API and tracks it for cleanup.
   * @param {object} [overrides] - Partial body to merge with defaults.
   * @returns {Promise<object>} The created partition object.
   */
  async create(overrides = {}) {
    const ts = Date.now();
    const body = {
      Name: `e2e-partition-${ts}`,
      Description: `E2E test partition ${ts}`,
      ...overrides,
    };

    const response = await this.request.put(`${this.baseURL}/v1/partition`, {
      headers: this.headers,
      data: body,
    });

    expect(response.ok(), `Partition creation failed: ${response.status()}`).toBeTruthy();
    const created = await response.json();
    this._createdNames.push(created.Name);
    return created;
  }

  /**
   * Reads a partition by name via the API.
   * @param {string} name
   * @returns {Promise<object>}
   */
  async read(name) {
    const response = await this.request.get(`${this.baseURL}/v1/partition/${name}`, {
      headers: this.headers,
    });
    expect(response.ok(), `Partition read failed: ${response.status()}`).toBeTruthy();
    return response.json();
  }

  /**
   * Updates a partition's description via the API.
   * @param {string} name
   * @param {object} updates - Fields to update (e.g. { Description: '...' })
   * @returns {Promise<object>}
   */
  async update(name, updates = {}) {
    const response = await this.request.put(`${this.baseURL}/v1/partition/${name}`, {
      headers: this.headers,
      data: updates,
    });
    expect(response.ok(), `Partition update failed: ${response.status()}`).toBeTruthy();
    return response.json();
  }

  /**
   * Deletes a partition by name via the API.
   * @param {string} name
   */
  async delete(name) {
    const response = await this.request.delete(`${this.baseURL}/v1/partition/${name}`, {
      headers: this.headers,
    });
    // 404 is acceptable — partition may already be gone.
    if (!response.ok() && response.status() !== 404) {
      console.warn(`Warning: failed to delete partition ${name} (${response.status()})`);
    }
    this._createdNames = this._createdNames.filter((n) => n !== name);
  }

  /**
   * Lists all partitions via the API.
   * @returns {Promise<object[]>}
   */
  async list() {
    const response = await this.request.get(`${this.baseURL}/v1/partitions`, {
      headers: this.headers,
    });
    expect(response.ok(), `Partition list failed: ${response.status()}`).toBeTruthy();
    return response.json();
  }

  /**
   * Deletes all partitions created by this helper instance.
   */
  async cleanup() {
    for (const name of [...this._createdNames]) {
      await this.delete(name);
    }
  }
}

/**
 * PartitionsPage — page-object helper for the Admin Partitions list and edit pages.
 */
class PartitionsPage {
  constructor(page, baseURL) {
    this.page = page;
    this.baseURL = baseURL;
  }

  // ── Navigation ─────────────────────────────────────────────────────────────

  async goto(dc = 'dc1') {
    await this.page.goto(`${this.baseURL}/ui/${dc}/partitions`, {
      waitUntil: 'domcontentloaded',
    });
  }

  async gotoEdit(partitionName, dc = 'dc1') {
    await this.page.goto(`${this.baseURL}/ui/${dc}/partitions/${partitionName}`, {
      waitUntil: 'domcontentloaded',
    });
  }

  // ── Form helpers ───────────────────────────────────────────────────────────

  get descriptionInput() {
    return this.page.getByRole('textbox', { name: 'Description (Optional)' });
  }

  get nameInput() {
    return this.page.getByRole('textbox', { name: 'Name' });
  }

  get saveButton() {
    return this.page.getByRole('button', { name: 'Save' });
  }

  get cancelButton() {
    return this.page.getByRole('button', { name: 'Cancel' });
  }

  get searchInput() {
    return this.page
      .getByPlaceholder('Search')
      .or(this.page.locator('input[type="search"]'))
      .first();
  }

  get searchAcrossButton() {
    return this.page.getByRole('button', { name: 'Search Across' });
  }

  get heading() {
    return this.page.getByRole('heading', { name: 'Admin Partitions' });
  }

  // ── List helpers ───────────────────────────────────────────────────────────

  /**
   * Returns the link for a named partition row.
   * @param {string} partitionName
   */
  partitionRow(partitionName) {
    return this.page.locator(`[data-test-partition="${partitionName}"]`);
  }

  /**
   * Returns the "More" overflow menu button for a named partition row.
   * @param {string} partitionName
   */
  moreButtonForPartition(partitionName) {
    return this.page
      .locator('[data-test-list-row]')
      .filter({ has: this.page.locator(`[data-test-partition="${partitionName}"]`) })
      .getByRole('button', { name: 'More' });
  }

  /**
   * Waits for a partition row to appear in the list.
   * @param {string} partitionName
   */
  async waitForPartitionInList(partitionName) {
    await expect(this.partitionRow(partitionName)).toBeVisible({ timeout: 15000 });
  }

  /**
   * Opens the "More > Edit" action for a named partition.
   * @param {string} partitionName
   */
  async openEditViaMoreMenu(partitionName) {
    await this.moreButtonForPartition(partitionName).click();
    await this.page.getByRole('menuitem', { name: 'Edit' }).click();
    await expect(this.page).toHaveURL(new RegExp(`/partitions/${partitionName}`), {
      timeout: 15000,
    });
  }

  /**
   * Fills description and saves the partition edit form.
   * @param {string} description
   */
  async fillDescriptionAndSave(description) {
    await this.descriptionInput.waitFor({ state: 'visible', timeout: 10000 });
    await this.descriptionInput.fill(description);
    await this.saveButton.click();
    await expect(this.page).toHaveURL(/\/partitions$/, { timeout: 20000 });
  }

  /**
   * Breadcrumb link that navigates back from an edit page to the list.
   */
  get allPartitionsBreadcrumb() {
    return this.page.getByRole('link', { name: 'All Admin Partitions' });
  }
}

/**
 * Extended test object providing `partitionApi` and `partitionsPage` fixtures.
 *
 * Usage:
 *   const { test, expect } = require('./fixtures');
 *   test('my test', async ({ page, partitionApi, partitionsPage }) => { ... });
 */
const test = base.extend({
  /**
   * Provides a PartitionApiHelper scoped to the test.
   * Automatically deletes any partitions created during the test.
   */
  partitionApi: async ({ request, baseURL }, use) => {
    const helper = new PartitionApiHelper(request, baseURL);
    await use(helper);
    await helper.cleanup();
  },

  /**
   * Provides a PartitionsPage page-object scoped to the test.
   */
  partitionsPage: async ({ page, baseURL }, use) => {
    const partitionsPage = new PartitionsPage(page, baseURL);
    await use(partitionsPage);
  },

  /**
   * Re-exports skipIfCommunityEdition bound to the test context for convenience.
   * Usage: await skipEnt(request, baseURL)
   */
  skipEnt: async ({ request, baseURL }, use) => {
    await use((req, url) => skipIfCommunityEdition(test, req ?? request, url ?? baseURL));
  },
});

module.exports = { test, expect, PartitionApiHelper, PartitionsPage };
