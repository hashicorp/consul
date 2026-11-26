/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

// Native QUnit port of tests/acceptance/dc/acls/roles/sorting.feature.

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
  assertPropertiesVertically,
} from 'consul-ui/tests/helpers/acceptance';

module('Acceptance | dc / acls / roles / sorting', function (hooks) {
  setupAcceptanceTest(hooks);

  nspaceScenario('Scenario: Sorting Roles', async function (assert, nspace) {
    createModels('dc', 1, 'dc-1');
    createModels('role', 4, [
      { Name: 'system-A', CreateIndex: 3 },
      { Name: 'system-D', CreateIndex: 2 },
      { Name: 'system-C', CreateIndex: 1 },
      { Name: 'system-B', CreateIndex: 4 },
    ]);

    await visit('roles', { dc: 'dc-1' }, { nspace });
    assert.equal(currentURL(), nspaceURL(nspace, '/dc-1/acls/roles'));
    assertVisibleCount(assert, page().roles, 4, 'role models');

    const sortBy = async function (option, expected) {
      await page().sort.selected();
      await page().sort.options.objectAt(option).button();
      assertPropertiesVertically(assert, page().roles, 'name', expected, 'roles');
    };
    await sortBy(1, ['system-D', 'system-C', 'system-B', 'system-A']);
    await sortBy(0, ['system-A', 'system-B', 'system-C', 'system-D']);
    await sortBy(3, ['system-C', 'system-D', 'system-A', 'system-B']);
    await sortBy(2, ['system-B', 'system-A', 'system-D', 'system-C']);
  });
});
