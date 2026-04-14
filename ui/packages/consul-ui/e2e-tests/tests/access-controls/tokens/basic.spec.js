/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

const { test, expect } = require('@playwright/test');

/**
 * Tokens - Basic Tests
 *
 * Auth is handled globally via storageState in Playwright config.
 */

test.describe('Access Controls - Tokens - Basic', () => {
  test('creates a token and opens token details', async ({ page, baseURL }) => {
    const description = `E2E token ${Date.now()}`;

    // Access dc2 on port 8501 (secondary datacenter from consul-ui-testing)
    const dc2URL = baseURL.replace(':8500', ':8501');
    await page.goto(`${dc2URL}/ui/dc2/services`, { waitUntil: 'domcontentloaded' });

    await page.getByRole('link', { name: 'Tokens' }).click();
    await page.getByRole('link', { name: 'Create' }).click();

    const descriptionInput = page.getByRole('textbox', { name: 'Description (Optional)' });
    await descriptionInput.waitFor({ state: 'visible', timeout: 30000 });
    await descriptionInput.fill(description);

    await page.getByRole('button', { name: 'Save' }).click();

    const createdTokenRow = page.getByText(description).first();
    await expect(createdTokenRow).toBeVisible({ timeout: 30000 });

    await createdTokenRow.click();

    await expect(page).toHaveURL(/\/tokens\//, { timeout: 30000 });
    await expect(page.getByRole('textbox', { name: 'Description (Optional)' })).toHaveValue(
      description
    );
  });
});
