/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

const { mergeTests, expect } = require('@playwright/test');
const { test: policyTest } = require('./policies/fixtures');
const { test: roleTest } = require('./roles/fixtures');
const { test: tokenTest } = require('./tokens/fixtures');
const { loginWithToken } = require('../../utils/auth-utils');

const test = mergeTests(policyTest, roleTest, tokenTest);

function managementTokenFromEnv() {
  return process.env.CONSUL_UI_TEST_TOKEN;
}

test.describe('Access Controls - Basic', () => {
  test('policy -> role -> token workflow with permission switch and cleanup', async ({
    page,
    baseURL,
    policyApi,
    roleApi,
    tokenApi,
    policiesPage,
    rolesPage,
    tokensPage,
  }) => {
    const mgmtToken = managementTokenFromEnv();
    if (!mgmtToken) {
      throw new Error('Set CONSUL_UI_TEST_TOKEN before running this test');
    }

    // Set tokens for API helpers
    policyApi.token = mgmtToken;
    roleApi.token = mgmtToken;
    tokenApi.token = mgmtToken;

    const unique = Date.now();
    const policyName = `e2e-policy-${unique}`;
    const roleName = `e2e-role-${unique}`;
    const tokenDescription = `e2e-token-${unique}`;

    let tokenDescriptionCurrent = tokenDescription;

    try {
      // 1-2. Navigate to Policies and create a policy.
      await policiesPage.gotoCreate();
      await policiesPage.fillAndSave(policyName);
      await expect(page).toHaveURL(/\/acls\/policies(?:$|\?)/, { timeout: 30000 });
      await policiesPage.waitForPolicyInList(policyName);

      // 3-4. Navigate to Roles and create a role with only that policy.
      await rolesPage.gotoCreate();
      await rolesPage.fillAndSave(roleName, {
        description: 'role-initial',
        policies: [policyName],
      });
      await expect(page).toHaveURL(/\/acls\/roles(?:$|\?)/, { timeout: 30000 });
      await rolesPage.waitForRoleInList(roleName);

      // 5. Create token with only that role.
      await tokensPage.gotoCreate();
      await tokensPage.fillAndSave(tokenDescription, { roles: [roleName] });
      await expect(page).toHaveURL(/\/acls\/tokens(?:$|\?)/, { timeout: 30000 });
      await tokensPage.waitForTokenInList(tokenDescriptionCurrent);

      // 6. Edit role, policy, token and verify persistence.
      await policiesPage.goto();
      await policiesPage.openPolicy(policyName);
      await policiesPage.editAndSave({ description: 'policy-updated' });
      await page.reload({ waitUntil: 'domcontentloaded' });
      await expect(policiesPage.descriptionInput).toHaveValue('policy-updated');

      await rolesPage.goto();
      await rolesPage.openRole(roleName);
      await rolesPage.editAndSave({ description: 'role-updated' });
      await expect(page).toHaveURL(/\/acls\/roles(?:$|\?)/, { timeout: 30000 });
      await rolesPage.openRole(roleName);
      await expect(rolesPage.descriptionInput).toHaveValue('role-updated');

      await tokensPage.goto();
      await tokensPage.openToken(tokenDescriptionCurrent);
      tokenDescriptionCurrent = `${tokenDescription}-updated`;
      await tokensPage.editAndSave({ description: tokenDescriptionCurrent });
      await expect(page).toHaveURL(/\/acls\/tokens(?:$|\?)/, { timeout: 30000 });
      await tokensPage.openToken(tokenDescriptionCurrent);
      await expect(tokensPage.descriptionInput).toHaveValue(tokenDescriptionCurrent);

      // 7. In token list, select token and use it.
      await tokensPage.goto();
      await tokensPage.useTokenFromList(tokenDescriptionCurrent);

      // 8. Verify we no longer have permission to create tokens.
      await tokensPage.goto();
      await expect(page.getByRole('link', { name: 'Create' })).toHaveCount(0);

      // 9. Sign out, then sign back in with management token.
      await page.getByLabel('Auth menu').click();
      await page.getByRole('button', { name: 'Log out'}).click();
      await loginWithToken(page, mgmtToken, baseURL);

      await tokensPage.goto();
      await expect(page.getByRole('link', { name: 'Create' })).toBeVisible({ timeout: 30000 });

      // 10. Cleanup in management context.
      await tokenApi.deleteByDescription(tokenDescriptionCurrent);
      await roleApi.deleteByName(roleName);
      await policyApi.deleteByName(policyName);
      tokenDescriptionCurrent = null;
    } finally {
      if (tokenDescriptionCurrent) {
        await tokenApi.deleteByDescription(tokenDescriptionCurrent).catch(() => {});
      }
      await roleApi.deleteByName(roleName).catch(() => {});
      await policyApi.deleteByName(policyName).catch(() => {});
    }
  });
});
