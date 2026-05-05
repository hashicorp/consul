/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

const { test, expect } = require('@playwright/test');

/**
 * Tokens - Basic Tests
 *
 * Comprehensive CRUD tests for ACL Tokens
 * Auth is handled globally via storageState in Playwright config.
 */

test.describe('Access Controls - Tokens - Basic', () => {
  test.beforeEach(async ({ page }) => {
    // Navigate to Tokens page before each test
    await page.goto('/ui/dc1/acls/tokens');
  });

  test('should navigate to Tokens page', async ({ page }) => {
    // Verify we're on the tokens page
    await expect(page).toHaveURL(/\/acls\/tokens/);

    // Verify the Create link is visible
    const createLink = page.getByRole('link', { name: 'Create' });
    await expect(createLink).toBeVisible();
  });

  test('should create a token with description only', async ({ page }) => {
    const description = `E2E Test Token ${Date.now()}`;

    await page.getByRole('link', { name: 'Create' }).click();
    await expect(page).toHaveURL(/\/tokens\/create/);

    const descriptionInput = page.getByRole('textbox', { name: 'Description (Optional)' });
    await descriptionInput.waitFor({ state: 'visible', timeout: 10000 });
    await descriptionInput.fill(description);

    await page.getByRole('button', { name: 'Save' }).click();

    // Verify token appears in the list
    await expect(page).toHaveURL(/\/acls\/tokens$/);
    await expect(page.getByText(description).first()).toBeVisible({ timeout: 10000 });
  });

  test('should view token details', async ({ page }) => {
    const description = `E2E View Token ${Date.now()}`;

    // Create token
    await page.getByRole('link', { name: 'Create' }).click();
    const descriptionInput = page.getByRole('textbox', { name: 'Description (Optional)' });
    await descriptionInput.waitFor({ state: 'visible', timeout: 10000 });
    await descriptionInput.fill(description);
    await page.getByRole('button', { name: 'Save' }).click();
    await expect(page).toHaveURL(/\/acls\/tokens$/);

    // Click on the token to view details
    const tokenRow = page.getByText(description).first();
    await expect(tokenRow).toBeVisible({ timeout: 10000 });
    await tokenRow.click();

    // Verify we're on the token details page
    await expect(page).toHaveURL(/\/tokens\//);

    // Verify token details are visible
    await expect(page.getByText('AccessorID')).toBeVisible();
    await expect(page.getByText('Token', { exact: true })).toBeVisible();
  });

  test('should delete a token', async ({ page }) => {
    const description = `E2E Delete Token ${Date.now()}`;

    // Create token
    await page.getByRole('link', { name: 'Create' }).click();
    const descriptionInput = page.getByRole('textbox', { name: 'Description (Optional)' });
    await descriptionInput.waitFor({ state: 'visible', timeout: 10000 });
    await descriptionInput.fill(description);
    await page.getByRole('button', { name: 'Save' }).click();
    await expect(page).toHaveURL(/\/acls\/tokens$/);

    // Click on the token
    const tokenRow = page.getByText(description).first();
    await expect(tokenRow).toBeVisible({ timeout: 10000 });
    await tokenRow.click();

    // Verify we're on the token details page
    await expect(page).toHaveURL(/\/tokens\//);

    // Delete the token
    await page.getByRole('button', { name: 'Delete' }).click();
    await page.getByRole('button', { name: 'Confirm Delete' }).click();

    // Verify redirect to tokens list
    await expect(page).toHaveURL(/\/acls\/tokens$/);

    // Verify token is no longer in the list
    await expect(page.getByText(description)).not.toBeVisible({ timeout: 5000 });
  });

  test('should cancel token creation', async ({ page }) => {
    const description = `E2E Cancel Token ${Date.now()}`;

    await page.getByRole('link', { name: 'Create' }).click();
    await expect(page).toHaveURL(/\/tokens\/create/);

    // Fill in some data
    const descriptionInput = page.getByRole('textbox', { name: 'Description (Optional)' });
    await descriptionInput.waitFor({ state: 'visible', timeout: 10000 });
    await descriptionInput.fill(description);

    // Click Cancel
    await page.getByRole('button', { name: 'Cancel' }).click();

    // Verify we're back on tokens list
    await expect(page).toHaveURL(/\/acls\/tokens$/);

    // Verify token was not created
    await expect(page.getByText(description)).not.toBeVisible();
  });
});

// Made with Bob
