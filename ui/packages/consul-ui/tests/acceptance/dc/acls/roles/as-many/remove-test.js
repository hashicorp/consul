/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module } from 'qunit';

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
  assertVisibleCount,
  requestMade,
} from 'consul-ui/tests/helpers/acceptance';

module('Acceptance | dc / acls / roles / as-many / remove: Remove', function (hooks) {
  setupAcceptanceTest(hooks);

  nspaceScenario('Scenario: Removing a role from a token', async function (assert, nspace) {
    createModels('dc', 1, 'datacenter');
    createModels('token', 1, {
      AccessorID: 'key',
      Roles: [{ Name: 'Role', ID: '00000000-0000-0000-0000-000000000001' }],
    });

    await visit('token', { dc: 'datacenter', token: 'key' }, { nspace });
    assert.equal(currentURL(), nspaceURL(nspace, '/datacenter/acls/tokens/key'));
    assertVisibleCount(assert, page().roles.selectedOptions, 1, 'role models');

    const role = page().roles.selectedOptions.objectAt(0);
    await role.actions();
    await role.delete();
    await role.confirmDelete();
    assertVisibleCount(assert, page().roles.selectedOptions, 0, 'role models');

    await submit();

    requestMade(
      assert,
      'PUT',
      nspaceEndpoint('/v1/acl/token/key?dc=datacenter&ns=@!namespace', nspace),
      { body: { Roles: [] } }
    );
    assert.equal(currentURL(), nspaceURL(nspace, '/datacenter/acls/tokens'));
  });
});
