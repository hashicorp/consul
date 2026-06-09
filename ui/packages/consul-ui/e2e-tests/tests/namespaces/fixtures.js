/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

const { test: base, expect } = require('@playwright/test');
const { skipIfCommunityEdition } = require('../../utils/ent-utils');

class NamespaceApiHelper {
  constructor(request, baseURL) {
    this.request = request;
    this.baseURL = baseURL;
    this.token = process.env.CONSUL_UI_TEST_TOKEN;
    this._createdNames = [];
  }

  get headers() {
    return { 'X-Consul-Token': this.token };
  }

  async read(name) {
    const response = await this.request.get(`${this.baseURL}/v1/namespace/${name}`, {
      headers: this.headers,
    });
    expect(response.ok(), `Namespace read failed: ${response.status()}`).toBeTruthy();
    return response.json();
  }

  async update(name, updates = {}) {
    const response = await this.request.put(`${this.baseURL}/v1/namespace/${name}`, {
      headers: this.headers,
      data: updates,
    });
    expect(response.ok(), `Namespace update failed: ${response.status()}`).toBeTruthy();
    return response.json();
  }

  async delete(name) {
    const response = await this.request.delete(`${this.baseURL}/v1/namespace/${name}`, {
      headers: this.headers,
    });
    if (!response.ok() && response.status() !== 404) {
      console.warn(`Warning: failed to delete namespace ${name} (${response.status()})`);
    }
    this._createdNames = this._createdNames.filter((n) => n !== name);
  }

  async cleanup() {
    for (const name of [...this._createdNames]) {
      await this.delete(name);
    }
  }
}

class NamespacesPage {
  constructor(page, baseURL) {
    this.page = page;
    this.baseURL = baseURL;
  }

  async goto(dc = 'dc1') {
    await this.page.goto(`${this.baseURL}/ui/${dc}/namespaces`, { waitUntil: 'domcontentloaded' });
  }

  async gotoEdit(namespaceName, dc = 'dc1') {
    await this.page.goto(`${this.baseURL}/ui/${dc}/namespaces/${namespaceName}`, {
      waitUntil: 'domcontentloaded',
    });
  }

  get nameInput() {
    return this.page.getByRole('textbox', { name: /Name/ });
  }

  get descriptionInput() {
    return this.page.getByRole('textbox', { name: 'Description (Optional)' });
  }

  get saveButton() {
    return this.page.getByRole('button', { name: 'Save' });
  }

  get cancelButton() {
    return this.page.getByRole('button', { name: 'Cancel' });
  }

  get heading() {
    return this.page.getByRole('heading', { name: 'Namespaces' });
  }

  get allNamespacesBreadcrumb() {
    return this.page.getByRole('link', { name: 'All Namespaces' });
  }

  moreButtonForNamespace(namespaceName) {
    return this.page
      .locator('[data-test-list-row]')
      .filter({ hasText: namespaceName })
      .getByRole('button', { name: 'More' });
  }

  async waitForNamespaceInList(namespaceName) {
    await expect(
      this.page.locator('[data-test-list-row]').filter({ hasText: namespaceName })
    ).toBeVisible({ timeout: 15000 });
  }

  async openEditViaMoreMenu(namespaceName) {
    await this.moreButtonForNamespace(namespaceName).click();
    await this.page.getByRole('menuitem', { name: 'Edit' }).click();
    await expect(this.page).toHaveURL(new RegExp(`/namespaces/${namespaceName}`), {
      timeout: 15000,
    });
  }

  async openDeleteViaMoreMenu(namespaceName) {
    await this.moreButtonForNamespace(namespaceName).click();
    await this.page.getByRole('menuitem', { name: 'Delete' }).click();
  }

  async confirmDeleteInModal() {
    await this.page
      .locator('#confirm-modal')
      .getByRole('button', { name: 'Delete' })
      .click({ timeout: 10000 });
  }

  async fillDescriptionAndSave(description) {
    await this.descriptionInput.waitFor({ state: 'visible', timeout: 10000 });
    await this.descriptionInput.fill(description);
    await this.saveButton.click();
    await expect(this.page).toHaveURL(/\/namespaces$/, { timeout: 20000 });
  }

  async selectFromSuperSelect(placeholder, optionText) {
    const field = this.page.getByText(placeholder).first();
    await field.waitFor({ state: 'visible', timeout: 15000 });
    await field.click();
    await field.fill(optionText);

    const option = this.page
      .getByRole('option', { name: optionText, exact: true })
      .or(this.page.getByText(optionText, { exact: true }))
      .first();
    await option.waitFor({ state: 'visible', timeout: 15000 });
    await option.click();
  }
}

const test = base.extend({
  namespaceApi: async ({ request, baseURL }, use) => {
    const helper = new NamespaceApiHelper(request, baseURL);
    await use(helper);
    await helper.cleanup();
  },

  namespacesPage: async ({ page, baseURL }, use) => {
    await use(new NamespacesPage(page, baseURL));
  },

  skipEnt: async ({ request, baseURL }, use) => {
    await use((req, url) => skipIfCommunityEdition(test, req ?? request, url ?? baseURL));
  },
});

module.exports = { test, expect, NamespaceApiHelper, NamespacesPage };
