/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

// Native QUnit port of tests/acceptance/dc/acls/policies/sorting.feature.

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

module('Acceptance | dc / acls / policies / sorting', function (hooks) {
  setupAcceptanceTest(hooks);

  nspaceScenario('Scenario: Sorting Policies', async function (assert, nspace) {
    createModels('dc', 1, 'dc-1');
    createModels('policy', 4, [
      { Name: 'system-A' },
      { Name: 'system-D' },
      { Name: 'system-C' },
      { Name: 'system-B' },
    ]);

    await visit('policies', { dc: 'dc-1' }, { nspace });
    assert.equal(currentURL(), nspaceURL(nspace, '/dc-1/acls/policies'));
    assertVisibleCount(assert, page().policies, 4, 'policy models');

    await page().sort.selected();
    await page().sort.options.objectAt(1).button();
    assertPropertiesVertically(
      assert,
      page().policies,
      'name',
      ['system-D', 'system-C', 'system-B', 'system-A'],
      'policies'
    );

    await page().sort.selected();
    await page().sort.options.objectAt(0).button();
    assertPropertiesVertically(
      assert,
      page().policies,
      'name',
      ['system-A', 'system-B', 'system-C', 'system-D'],
      'policies'
    );
  });
});
