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
  // Track created token IDs for cleanup
  const createdTokenIds = [];

  // Cleanup function to delete ALL E2E test tokens via API
  async function cleanupAllE2ETokens(request, baseURL) {
    console.log('\n🧹 Cleaning up all E2E test tokens...');

    const token = process.env.CONSUL_UI_TEST_TOKEN;

    try {
      // Get all tokens
      const listResponse = await request.get(`${baseURL}/v1/acl/tokens`, {
        headers: {
          'X-Consul-Token': token,
        },
      });

      if (!listResponse.ok()) {
        console.log('❌ Failed to list tokens');
        return;
      }

      const tokens = await listResponse.json();
      const e2eTokens = tokens.filter((t) => {
        const description = t.Description || '';
        return description.toLowerCase().includes('e2e');
      });

      console.log(`📋 Found ${e2eTokens.length} E2E tokens to delete\n`);

      let successCount = 0;
      for (const t of e2eTokens) {
        try {
          const response = await request.delete(`${baseURL}/v1/acl/token/${t.AccessorID}`, {
            headers: {
              'X-Consul-Token': token,
            },
          });
          if (response.ok() || response.status() === 404) {
            console.log(`  ✓ Deleted: ${t.Description} (${t.AccessorID})`);
            successCount++;
          }
        } catch (error) {
          console.log(`  ✗ Failed to delete: ${t.Description}`);
        }
      }

      console.log(`\n✅ Deleted ${successCount} E2E tokens\n`);
    } catch (error) {
      console.log('❌ Cleanup failed:', error.message);
    }

    createdTokenIds.length = 0;
  }

  // Cleanup at the end - delete ALL E2E tokens
  test.afterAll(async ({ request, baseURL }) => {
    await cleanupAllE2ETokens(request, baseURL);
  });

  test('creates a token and opens token details', async ({ page, baseURL }) => {
    const description = `E2E token ${Date.now()}`;

    await page.goto(`${baseURL}/ui/dc1/services`, { waitUntil: 'domcontentloaded' });

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

    // Extract token ID from URL for cleanup
    const url = page.url();
    const tokenIdMatch = url.match(/\/tokens\/([^\/]+)/);
    if (tokenIdMatch) {
      createdTokenIds.push(tokenIdMatch[1]);
      console.log(`✓ Created token: ${tokenIdMatch[1]}`);
    }
  });

  test('creates a token and deletes it from token details page', async ({ page, baseURL }) => {
    const description = `E2E Test Token ${Date.now()}`;

    await page.goto(`${baseURL}/ui/dc1/services`, { waitUntil: 'domcontentloaded' });

    // Navigate to Tokens page
    await page.getByRole('link', { name: 'Tokens' }).click();
    await page.getByRole('link', { name: 'Create' }).click();

    // Fill in token description
    const descriptionInput = page.getByRole('textbox', { name: 'Description (Optional)' });
    await descriptionInput.waitFor({ state: 'visible', timeout: 30000 });
    await descriptionInput.fill(description);

    // Save the token
    await page.getByRole('button', { name: 'Save' }).click();

    // Wait for token to be created and visible in the list
    const createdTokenRow = page.getByText(description).first();
    await expect(createdTokenRow).toBeVisible({ timeout: 30000 });

    // Click on the token to open details page
    await createdTokenRow.click();

    // Verify we're on the token details page
    await expect(page).toHaveURL(/\/tokens\//, { timeout: 30000 });
    await expect(page.getByRole('textbox', { name: 'Description (Optional)' })).toHaveValue(
      description
    );

    // Delete the token from the details page
    await page.getByRole('button', { name: 'Delete' }).click();
    
    // Confirm deletion
    await page.getByRole('button', { name: 'Confirm Delete' }).click();

    // Verify we're redirected back to tokens list
    await expect(page).toHaveURL(/\/tokens$/, { timeout: 30000 });

    // Verify the token is no longer in the list
    await expect(page.getByText(description)).not.toBeVisible({ timeout: 10000 });

    console.log(`✓ Created and deleted token: ${description}`);
  });

});
