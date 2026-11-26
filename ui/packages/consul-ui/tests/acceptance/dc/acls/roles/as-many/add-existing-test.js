/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

// Native QUnit port of tests/acceptance/dc/acls/roles/as-many/add-existing.feature.
// Its single scenario was untitled (reported as "Scenario: "), so the native
// test is given a title.

import { module } from 'qunit';
import { click, fillIn } from '@ember/test-helpers';

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

module('Acceptance | dc / acls / roles / as many / add existing: Add existing', function (hooks) {
  setupAcceptanceTest(hooks);

  nspaceScenario('Scenario: Adding existing roles to a token', async function (assert, nspace) {
    createModels('dc', 1, 'datacenter');
    createModels('token', 1, { AccessorID: 'key', Description: 'The Description', Roles: null });
    createModels('role', 2, [
      { ID: 'role-2', Name: 'Role 2' },
      { ID: 'role-1', Name: 'Role 1' },
    ]);

    await visit('token', { dc: 'datacenter', token: 'key' }, { nspace });
    assert.equal(currentURL(), nspaceURL(nspace, '/datacenter/acls/tokens/key'));

    await click('form > #roles .ember-power-select-trigger');
    assert.dom('.ember-power-select-option:first-child').includesText('Role 1');
    await fillIn('.ember-power-select-search-input', 'Role 1');
    await click('.ember-power-select-option:first-child');
    assertVisibleCount(assert, page().roles.selectedOptions, 1, 'role models');

    await click('form > #roles .ember-power-select-trigger');
    await fillIn('.ember-power-select-search-input', 'Role 2');
    await click('.ember-power-select-option:first-child');
    assertVisibleCount(assert, page().roles.selectedOptions, 2, 'role models');

    await fillIn('[name="Description"]', 'The Description');
    await submit();

    requestMade(
      assert,
      'PUT',
      nspaceEndpoint('/v1/acl/token/key?dc=datacenter&ns=@!namespace', nspace),
      {
        body: {
          Description: 'The Description',
          Roles: [
            { ID: 'role-1', Name: 'Role 1' },
            { ID: 'role-2', Name: 'Role 2' },
          ],
        },
      }
    );
    assert.equal(currentURL(), nspaceURL(nspace, '/datacenter/acls/tokens'));
    assert.dom('[data-notification]').hasClass('hds-toast');
    assert.dom('[data-notification]').hasClass('hds-alert--color-success');
  });
});
