const { test, expect } = require('@playwright/test');

/**
 * Overview - Basic Tests
 *
 * Fast, essential tests for Overview/Dashboard
 * Run on every PR
 */

test.describe('Overview - Basic Tests', () => {
  test('overview page loads', async ({ page }) => {
    await page.goto('http://localhost:4200');

    // Verify dashboard loads
    await expect(page.locator('h1')).toBeVisible();
  });

  // TODO: Add more basic tests
  // - Verify key metrics displayed
  // - Check navigation links work
});

// Made with Bob
