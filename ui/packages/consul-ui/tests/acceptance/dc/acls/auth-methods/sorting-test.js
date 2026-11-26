/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

// Native QUnit port of tests/acceptance/dc/acls/auth-methods/sorting.feature.

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

module('Acceptance | dc / acls / auth-methods / sorting', function (hooks) {
  setupAcceptanceTest(hooks);

  nspaceScenario('Scenario: Sorting Auth Methods', async function (assert, nspace) {
    createModels('dc', 1, 'dc-1');
    createModels('authMethod', 4, [
      { Name: 'system-A', DisplayName: '' },
      { Name: 'system-D', DisplayName: '' },
      { Name: 'system-C', DisplayName: '' },
      { Name: 'system-B', DisplayName: '' },
    ]);

    await visit('authMethods', { dc: 'dc-1' }, { nspace });
    assert.equal(currentURL(), nspaceURL(nspace, '/dc-1/acls/auth-methods'));
    assertVisibleCount(assert, page().authMethods, 4, 'authMethod models');

    await page().sort.selected();
    await page().sort.options.objectAt(1).button();
    assertPropertiesVertically(
      assert,
      page().authMethods,
      'name',
      ['system-D', 'system-C', 'system-B', 'system-A'],
      'authMethods'
    );

    await page().sort.selected();
    await page().sort.options.objectAt(0).button();
    assertPropertiesVertically(
      assert,
      page().authMethods,
      'name',
      ['system-A', 'system-B', 'system-C', 'system-D'],
      'authMethods'
    );
  });
});
