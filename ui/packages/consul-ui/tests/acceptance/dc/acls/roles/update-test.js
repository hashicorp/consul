/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

// Native QUnit port of tests/acceptance/dc/acls/roles/update.feature.
//
// The `.feature` titled its Where-table scenario "Update to [Name], [Rules],
// [Description]", but the table had no Rules column, so "[Rules]" appeared
// literally in every title. The native titles leave it out.

import { module } from 'qunit';

import {
  setupAcceptanceTest,
  nspaceScenario,
  nspaceURL,
  nspaceEndpoint,
  createModels,
  respondWith,
  visit,
  page,
  submit,
  currentURL,
  fillInForm,
  assertVisibleCount,
  requestMade,
  requestMadeWithoutProperties,
} from 'consul-ui/tests/helpers/acceptance';

// Background: the edit form for the "role-id" role, used by 3 tokens.
const visitEdit = async function (assert, nspace) {
  createModels('dc', 1, 'datacenter');
  createModels('role', 1, { ID: 'role-id' });
  createModels('token', 3);

  await visit('role', { dc: 'datacenter', role: 'role-id' }, { nspace });

  assert.equal(currentURL(), nspaceURL(nspace, '/datacenter/acls/roles/role-id'));
  assertVisibleCount(assert, page().tokens, 3, 'token models');
  assert.equal(document.title, 'Edit Role - Consul');
};

const rows = [
  { Name: 'role-name', Description: 'role-name description' },
  { Name: 'role', Description: 'role name description' },
  { Name: 'roleName', Description: 'role%20name description' },
];

module('Acceptance | dc / acls / roles / update: ACL Role Update', function (hooks) {
  setupAcceptanceTest(hooks);

  rows.forEach(({ Name, Description }) => {
    nspaceScenario(`Scenario: Update to ${Name}, ${Description}`, async function (assert, nspace) {
      await visitEdit(assert, nspace);

      await fillInForm('role', { Name, Description });
      await submit();

      requestMade(
        assert,
        'PUT',
        nspaceEndpoint('/v1/acl/role/role-id?dc=datacenter&ns=@!namespace', nspace),
        { body: { Name, Description } }
      );
      assert.equal(currentURL(), nspaceURL(nspace, '/datacenter/acls/roles'));
      assert.dom('[data-notification]').hasClass('hds-toast');
      assert.dom('[data-notification]').hasClass('hds-alert--color-success');
    });
  });

  nspaceScenario('Scenario: There was an error saving the key', async function (assert, nspace) {
    await visitEdit(assert, nspace);

    respondWith('/v1/acl/role/role-id', { status: 500 });
    await submit();

    assert.equal(currentURL(), nspaceURL(nspace, '/datacenter/acls/roles/role-id'));
    assert.dom('[data-notification]').hasClass('hds-toast');
    assert.dom('[data-notification]').hasClass('hds-alert--color-critical');
  });

  nspaceScenario(
    'Scenario: Updating a simple ACL role when Namespaces are disabled does not send Namespace',
    async function (assert, nspace) {
      await visitEdit(assert, nspace);

      await fillInForm('role', { Description: 'Description' });
      await submit();

      requestMadeWithoutProperties(assert, 'PUT', '/v1/acl/role/role-id?dc=datacenter', [
        'Namespace',
      ]);
      assert.equal(currentURL(), nspaceURL(nspace, '/datacenter/acls/roles'));
      assert.dom('[data-notification]').hasClass('hds-toast');
      assert.dom('[data-notification]').hasClass('hds-alert--color-success');
    },
    { notNamespaceable: true }
  );
});
