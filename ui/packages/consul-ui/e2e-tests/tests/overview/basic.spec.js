/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

const { test, expect } = require('@playwright/test');

/**
 * Overview - Basic Tests
 *
 * Fast, essential tests for Overview/Dashboard
 * Run on every PR
 */

test.describe('Overview - Basic Tests', () => {
  test('overview page loads', async ({ page }) => {
    // Use baseURL from config (can be overridden by PLAYWRIGHT_BASE_URL env var)
    await page.goto('/');

    // Verify dashboard loads
    await expect(page.locator('h1')).toBeVisible();
  });

  // TODO: Add more basic tests
  // - Verify key metrics displayed
  // - Check navigation links work
});

// Made with Bob
