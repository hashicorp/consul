/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

// Native QUnit port of tests/acceptance/dc/acls/roles/as-many/list.feature.
// Its single scenario was untitled (reported as "Scenario: "), so the native
// test is given a title.

import { module } from 'qunit';

import {
  setupAcceptanceTest,
  nspaceScenario,
  nspaceURL,
  createModels,
  visit,
  page,
  currentURL,
  assertVisibleCount,
} from 'consul-ui/tests/helpers/acceptance';

module('Acceptance | dc / acls / roles / as-many / list: List', function (hooks) {
  setupAcceptanceTest(hooks);

  nspaceScenario('Scenario: Listing roles on the token page', async function (assert, nspace) {
    createModels('dc', 1, 'datacenter');
    createModels('token', 1, {
      AccessorID: 'key',
      Roles: [
        { Name: 'Role', ID: 0 },
        { Name: 'Role 2', ID: 2 },
        { Name: 'Role 3', ID: 3 },
      ],
    });

    await visit('token', { dc: 'datacenter', token: 'key' }, { nspace });
    assert.equal(currentURL(), nspaceURL(nspace, '/datacenter/acls/tokens/key'));

    assertVisibleCount(assert, page().roles.selectedOptions, 3, 'role models');
  });
});
