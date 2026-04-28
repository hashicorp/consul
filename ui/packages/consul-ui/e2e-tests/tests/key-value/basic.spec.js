/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

const { test, expect } = require('@playwright/test');

function kvValueEditor(page) {
  return page.locator('.cm-content[aria-label="value"]').first();
}

function kvRow(page, text) {
  return page.locator(`text=${text}`).first();
}

async function openKVCreate(page) {
  await page.getByRole('link', { name: 'Create' }).click();
  await expect(page).toHaveURL(/\/ui\/dc1\/kv(?:\/.*)?\/create/);
}

async function openKVKey(page, keyName) {
  const row = kvRow(page, keyName);
  await expect(row).toBeVisible({ timeout: 15000 });
  await row.click();
}

async function openNestedKVKey(page, segments, keyName) {
  for (const segment of segments) {
    const row = kvRow(page, segment);
    await expect(row).toBeVisible({ timeout: 15000 });
    await row.click();
  }

  await openKVKey(page, keyName);
}

async function waitForKVEdit(page, keyName) {
  await expect(page).toHaveURL(new RegExp(`/ui/dc1/kv/${keyName}/edit`), { timeout: 15000 });
}

async function fillKVValue(page, value, replace = false) {
  const editor = kvValueEditor(page);
  await editor.waitFor({ state: 'visible', timeout: 15000 });
  await editor.click();

  await page.keyboard.press(process.platform === 'darwin' ? 'Meta+A' : 'Control+A');
  if (!replace) {
    await page.keyboard.press('Backspace');
  }
  await page.keyboard.insertText(value);
}

/**
 * Key/Value - Basic Tests
 *
 * Fast, essential tests for Key/Value feature
 * Run on every PR
 */

// Helper function to delete a KV pair via API
async function deleteKVPair(page, keyName) {
  const token = process.env.CONSUL_UI_TEST_TOKEN;
  const baseURL = page.context()._options.baseURL || 'http://localhost:4200';

  try {
    const response = await page.request.delete(`${baseURL}/v1/kv/${keyName}?recurse=true`, {
      headers: {
        'X-Consul-Token': token,
      },
    });
    console.log(`Deleted KV: ${keyName} (status: ${response.status()})`);
  } catch (error) {
    console.log(`Note: Could not delete ${keyName} - ${error.message}`);
  }
}

// Helper function to create a KV pair via API
async function createKVPair(page, keyName, value = '') {
  const token = process.env.CONSUL_UI_TEST_TOKEN;
  const baseURL = page.context()._options.baseURL || 'http://localhost:4200';

  try {
    const response = await page.request.put(`${baseURL}/v1/kv/${keyName}`, {
      headers: {
        'X-Consul-Token': token,
      },
      data: value,
    });
    console.log(`Created KV: ${keyName} (status: ${response.status()})`);
    return response.ok();
  } catch (error) {
    console.log(`Note: Could not create ${keyName} - ${error.message}`);
    return false;
  }
}

// Helper function to read a KV pair via API
async function readKVPair(page, keyName) {
  const token = process.env.CONSUL_UI_TEST_TOKEN;
  const baseURL = page.context()._options.baseURL || 'http://localhost:4200';

  try {
    const response = await page.request.get(`${baseURL}/v1/kv/${keyName}?raw`, {
      headers: {
        'X-Consul-Token': token,
      },
    });
    if (response.ok()) {
      return await response.text();
    }
    return null;
  } catch (error) {
    console.log(`Note: Could not read ${keyName} - ${error.message}`);
    return null;
  }
}

test.describe('Key/Value - Basic Tests', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/ui/dc1/kv');
  });

  test('should navigate to Key/Value page', async ({ page }) => {
    await expect(page).toHaveURL(/\/ui\/dc1\/kv/);
    await expect(page.getByRole('link', { name: 'Create' })).toBeVisible();
  });

  test('should validate required key name field', async ({ page }) => {
    await openKVCreate(page);

    const saveButton = page.getByRole('button', { name: 'Save' });
    await expect(saveButton).toBeDisabled();
    await page.getByRole('textbox', { name: 'Key or folder To create a' }).fill('test-key');
    await expect(saveButton).toBeEnabled();
  });

  test('should create a key with value', async ({ page }) => {
    const keyName = 'e2e-test-key';

    try {
      await openKVCreate(page);

      await page.getByRole('textbox', { name: 'Key or folder To create a' }).fill(keyName);

      await fillKVValue(page, 'test-value');

      await page.getByRole('button', { name: 'Save' }).click();

      await expect(page).toHaveURL(/\/ui\/dc1\/kv/);
      await expect(page.locator(`text=${keyName}`)).toBeVisible();
    } finally {
      await deleteKVPair(page, keyName);
    }
  });

  test('should create a key with empty value', async ({ page }) => {
    const keyName = 'e2e-empty-key';

    try {
      await openKVCreate(page);
      await page.getByRole('textbox', { name: 'Key or folder To create a' }).fill(keyName);
      await page.getByRole('button', { name: 'Save' }).click();

      await expect(page).toHaveURL(/\/ui\/dc1\/kv/);
      await expect(page.locator(`text=${keyName}`)).toBeVisible();
    } finally {
      await deleteKVPair(page, keyName);
    }
  });

  test('should create a key with special characters and JSON value', async ({ page }) => {
    const keyName = 'e2e-key_with.special-chars';

    try {
      await openKVCreate(page);
      await page.getByRole('textbox', { name: 'Key or folder To create a' }).fill(keyName);

      const jsonValue = JSON.stringify({ name: 'test', value: 123, enabled: true }, null, 2);

      await fillKVValue(page, jsonValue);

      await page.getByRole('button', { name: 'Save' }).click();

      await expect(page).toHaveURL(/\/ui\/dc1\/kv/);
      await expect(page.locator(`text=${keyName}`)).toBeVisible();
    } finally {
      await deleteKVPair(page, keyName);
    }
  });

  test('should create a key with multiline value', async ({ page }) => {
    const keyName = 'e2e-multiline-key';

    try {
      await openKVCreate(page);
      await page.getByRole('textbox', { name: 'Key or folder To create a' }).fill(keyName);

      const multilineValue = 'Line 1\nLine 2\nLine 3\nLine 4';

      await fillKVValue(page, multilineValue);

      await page.getByRole('button', { name: 'Save' }).click();

      await expect(page).toHaveURL(/\/ui\/dc1\/kv/);
      await expect(page.locator(`text=${keyName}`)).toBeVisible();
    } finally {
      await deleteKVPair(page, keyName);
    }
  });

  test('should create nested keys', async ({ page }) => {
    const keyName = 'e2e-parent/child/grandchild';

    try {
      await openKVCreate(page);
      await page.getByRole('textbox', { name: 'Key or folder To create a' }).fill(keyName);

      await fillKVValue(page, 'nested-value');

      await page.getByRole('button', { name: 'Save' }).click();

      await expect(page).toHaveURL(/\/ui\/dc1\/kv/);
    } finally {
      await deleteKVPair(page, 'e2e-parent');
    }
  });

  test('should create folder and key inside it', async ({ page }) => {
    const folderPath = 'e2e-folder/subfolder/';

    try {
      await openKVCreate(page);
      await page.getByRole('textbox', { name: 'Key or folder To create a' }).fill(folderPath);
      await page.getByRole('button', { name: 'Save' }).click();
      await expect(page).toHaveURL(/\/ui\/dc1\/kv/);

      await openNestedKVKey(page, ['e2e-folder'], 'subfolder');
      await openKVCreate(page);
      await page.getByRole('textbox', { name: 'Key or folder To create a' }).fill('key-in-folder');

      await fillKVValue(page, 'value-in-folder');

      await page.getByRole('button', { name: 'Save' }).click();

      await expect(page.locator('text=key-in-folder')).toBeVisible();
    } finally {
      await deleteKVPair(page, 'e2e-folder');
    }
  });

  test('should read key value', async ({ page }) => {
    const keyName = 'e2e-read-key';
    const keyValue = 'test-value-to-read';

    try {
      await createKVPair(page, keyName, keyValue);
      await page.goto('/ui/dc1/kv');
      await openKVKey(page, keyName);

      await waitForKVEdit(page, keyName);

      const readValue = await readKVPair(page, keyName);
      expect(readValue).toBe(keyValue);
    } finally {
      await deleteKVPair(page, keyName);
    }
  });

  test('should read key in nested folder', async ({ page }) => {
    const keyName = 'e2e-read-folder/nested/test-key';
    const keyValue = 'nested-read-value';

    try {
      await createKVPair(page, keyName, keyValue);
      await page.goto('/ui/dc1/kv');
      await openNestedKVKey(page, ['e2e-read-folder', 'nested'], 'test-key');

      await waitForKVEdit(page, keyName);

      const readValue = await readKVPair(page, keyName);
      expect(readValue).toBe(keyValue);
    } finally {
      await deleteKVPair(page, 'e2e-read-folder');
    }
  });

  test('should update key value', async ({ page }) => {
    const keyName = 'e2e-update-key';
    const originalValue = 'original-value';
    const updatedValue = 'updated-value';

    try {
      await createKVPair(page, keyName, originalValue);
      await page.goto('/ui/dc1/kv');
      await openKVKey(page, keyName);

      const editButton = page.getByRole('button', { name: 'Edit' });
      if (await editButton.isVisible({ timeout: 2000 }).catch(() => false)) {
        await editButton.click();
      }

      await waitForKVEdit(page, keyName);
      await fillKVValue(page, updatedValue, true);

      await page.getByRole('button', { name: 'Save' }).click();

      const readValue = await readKVPair(page, keyName);
      expect(readValue).toBe(updatedValue);
    } finally {
      await deleteKVPair(page, keyName);
    }
  });

  test('should update key in nested folder', async ({ page }) => {
    const keyName = 'e2e-update-folder/nested/update-key';
    const originalValue = 'original-nested-value';
    const updatedValue = 'updated-nested-value';

    try {
      await createKVPair(page, keyName, originalValue);
      await page.goto('/ui/dc1/kv');

      await openNestedKVKey(page, ['e2e-update-folder', 'nested'], 'update-key');

      const editButton = page.getByRole('button', { name: 'Edit' });
      if (await editButton.isVisible({ timeout: 2000 }).catch(() => false)) {
        await editButton.click();
      }

      await waitForKVEdit(page, keyName);
      await fillKVValue(page, updatedValue, true);

      await page.getByRole('button', { name: 'Save' }).click();

      const readValue = await readKVPair(page, keyName);
      expect(readValue).toBe(updatedValue);
    } finally {
      await deleteKVPair(page, 'e2e-update-folder');
    }
  });

  test('should delete a key', async ({ page }) => {
    const keyName = 'e2e-delete-key';

    try {
      await createKVPair(page, keyName, 'value-to-delete');
      await page.goto('/ui/dc1/kv');
      await openKVKey(page, keyName);
      await page.getByRole('button', { name: 'Delete' }).click();

      const confirmButton = page.getByRole('button', { name: 'Confirm' });
      if (await confirmButton.isVisible({ timeout: 2000 }).catch(() => false)) {
        await confirmButton.click();
      }

      await expect(page).toHaveURL(/\/ui\/dc1\/kv/);
      await expect(kvRow(page, keyName)).not.toBeVisible();
    } catch (error) {
      await deleteKVPair(page, keyName);
      throw error;
    }
  });

  test('should delete a key in nested folder', async ({ page }) => {
    const keyName = 'e2e-delete-folder/nested/delete-key';
    const anotherKey = 'e2e-delete-folder/nested/another-key';

    try {
      await createKVPair(page, keyName, 'nested-value-to-delete');
      await createKVPair(page, anotherKey, 'another-value');

      await page.goto('/ui/dc1/kv');
      await openNestedKVKey(page, ['e2e-delete-folder'], 'nested');

      await expect(kvRow(page, 'delete-key')).toBeVisible();
      await expect(kvRow(page, 'another-key')).toBeVisible();

      await openKVKey(page, 'delete-key');
      await page.getByRole('button', { name: 'Delete' }).click();

      const confirmButton = page.getByRole('button', { name: 'Confirm' });
      if (await confirmButton.isVisible({ timeout: 2000 }).catch(() => false)) {
        await confirmButton.click();
      }

      await page.waitForURL(/\/ui\/dc1\/kv/, { timeout: 5000 });
      await expect(kvRow(page, 'delete-key')).not.toBeVisible();
      await expect(kvRow(page, 'another-key')).toBeVisible();
    } catch (error) {
      await deleteKVPair(page, 'e2e-delete-folder');
      throw error;
    } finally {
      await deleteKVPair(page, 'e2e-delete-folder');
    }
  });

  test('should delete folder via API', async ({ page }) => {
    const keyInFolder = 'e2e-delete-entire-folder/key1';

    try {
      await createKVPair(page, keyInFolder, 'value1');
      await page.goto('/ui/dc1/kv');
      await expect(kvRow(page, 'e2e-delete-entire-folder')).toBeVisible();

      await deleteKVPair(page, 'e2e-delete-entire-folder');
      await page.reload();

      await expect(kvRow(page, 'e2e-delete-entire-folder')).not.toBeVisible();
    } catch (error) {
      await deleteKVPair(page, 'e2e-delete-entire-folder');
      throw error;
    }
  });

  test('should delete nested folder structure via API', async ({ page }) => {
    const keyInFolder = 'e2e-delete-nested/level1/level2/key';

    try {
      await createKVPair(page, keyInFolder, 'nested-value');
      await page.goto('/ui/dc1/kv');
      await expect(kvRow(page, 'e2e-delete-nested')).toBeVisible();

      await deleteKVPair(page, 'e2e-delete-nested');
      await page.reload();

      await expect(kvRow(page, 'e2e-delete-nested')).not.toBeVisible();
    } catch (error) {
      await deleteKVPair(page, 'e2e-delete-nested');
      throw error;
    }
  });
});

// Made with Bob
