/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

// Native QUnit port of tests/acceptance/dc/acls/roles/as-many/add-new.feature.
// The feature was `@notNamespaceable` (CE only), as before. "Add Role and add a
// new Service Identity" was `@ignore`d and still fails, so it stays skipped.

import { module } from 'qunit';
import { click } from '@ember/test-helpers';

import {
  setupAcceptanceTest,
  nspaceScenario,
  nspaceURL,
  nspaceEndpoint,
  createModels,
  visit,
  page,
  submit,
  currentURL,
  fillInForm,
  requestMade,
} from 'consul-ui/tests/helpers/acceptance';

// The api-double gives each newly created policy/role this ID.
const NEW_ID = 'ee52203d-989f-4f7a-ab5a-2bef004164ca-1';

// Background: the token's edit page with the new-role form open and filled in.
const openNewRoleForm = async function (assert, nspace) {
  createModels('dc', 1, 'datacenter');
  createModels('token', 1, {
    AccessorID: 'key',
    Description: 'The Description',
    Roles: null,
    Policies: null,
    ServiceIdentities: null,
  });
  createModels('policy', 1, { ID: 'policy-1', Name: 'policy' });

  await visit('token', { dc: 'datacenter', token: 'key' }, { nspace });
  assert.equal(currentURL(), nspaceURL(nspace, '/datacenter/acls/tokens/key'));
  await page().roles.create();
  assert.dom('[data-test-role-creator]').exists();
  await fillInForm('role', { Name: 'New-Role', Description: 'New Role Description' });
};

const assertRoleSaved = function (assert, nspace, body) {
  requestMade(assert, 'PUT', nspaceEndpoint('/v1/acl/role?dc=datacenter&ns=@namespace', nspace), {
    body,
  });
};

const assertTokenSaved = async function (assert, nspace) {
  await submit();
  requestMade(
    assert,
    'PUT',
    nspaceEndpoint('/v1/acl/token/key?dc=datacenter&ns=@namespace', nspace),
    {
      body: {
        Description: 'The Description',
        Roles: [{ Name: 'New-Role', ID: NEW_ID }],
      },
    }
  );
  assert.equal(currentURL(), nspaceURL(nspace, '/datacenter/acls/tokens'));
  assert.dom('[data-notification]').hasClass('hds-toast');
  assert.dom('[data-notification]').hasClass('hds-alert--color-success');
};

module('Acceptance | dc / acls / roles / as-many / add-new: Add new', function (hooks) {
  setupAcceptanceTest(hooks);

  nspaceScenario(
    'Scenario: Add Policy-less Role',
    async function (assert, nspace) {
      await openNewRoleForm(assert, nspace);

      await page().roles.form.submit();
      assertRoleSaved(assert, nspace, { Name: 'New-Role', Description: 'New Role Description' });
      assert.dom('[data-test-role-creator]').doesNotExist();

      await assertTokenSaved(assert, nspace);
    },
    { notNamespaceable: true }
  );

  nspaceScenario(
    'Scenario: Add Role that has an existing Policy',
    async function (assert, nspace) {
      await openNewRoleForm(assert, nspace);

      await click('#new-role .ember-power-select-trigger');
      await click('.ember-power-select-option:first-child');
      await page().roles.form.submit();
      assertRoleSaved(assert, nspace, {
        Name: 'New-Role',
        Description: 'New Role Description',
        Policies: [{ ID: 'policy-1', Name: 'policy' }],
      });

      await assertTokenSaved(assert, nspace);
    },
    { notNamespaceable: true }
  );

  nspaceScenario(
    'Scenario: Add Role and add a new Policy',
    async function (assert, nspace) {
      await openNewRoleForm(assert, nspace);

      await page().roles.form.policies.create();
      assert.dom('[data-test-policy-form]').exists();
      await fillInForm('role[policy]', {
        Name: 'New-Policy',
        Description: 'New Policy Description',
        Rules: 'key {}',
      });
      await page().roles.form.policies.form.submit();
      requestMade(
        assert,
        'PUT',
        nspaceEndpoint('/v1/acl/policy?dc=datacenter&ns=@namespace', nspace),
        { body: { Name: 'New-Policy', Description: 'New Policy Description', Rules: 'key {}' } }
      );

      assert.dom("[data-test-role-form] [name='role[Name]']").exists();
      await page().roles.form.submit();
      assertRoleSaved(assert, nspace, {
        Name: 'New-Role',
        Description: 'New Role Description',
        Policies: [{ ID: NEW_ID, Name: 'New-Policy' }],
      });

      await assertTokenSaved(assert, nspace);
    },
    { notNamespaceable: true }
  );

  nspaceScenario(
    'Scenario: Add Role and add a new Service Identity',
    async function (assert, nspace) {
      await openNewRoleForm(assert, nspace);

      await page().roles.form.policies.create();
      await fillInForm('role[policy]', { Name: 'New-Service-Identity' });
      await click("[value='service-identity']");
      // The first submit is the popped-up policy form, due to the way things work.
      await page().roles.form.submit();
      await page().roles.form.submit();
      assertRoleSaved(assert, nspace, {
        Name: 'New-Role',
        Description: 'New Role Description',
        ServiceIdentities: [{ ServiceName: 'New-Service-Identity' }],
      });

      await assertTokenSaved(assert, nspace);
    },
    // FIXME: adding a new service identity fails (also skipped in
    // dc/acls/policies/as-many/add-new-test.js).
    { notNamespaceable: true, ignore: true }
  );

  nspaceScenario(
    'Scenario: Cancel the inline role form',
    async function (assert, nspace) {
      await openNewRoleForm(assert, nspace);

      await page().roles.form.cancel();
      assert.dom('[data-test-role-creator]').doesNotExist();
      assert.dom('[data-test-role-create]').exists();
    },
    { notNamespaceable: true }
  );
});
