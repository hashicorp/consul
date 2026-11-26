/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

// Native QUnit port of tests/acceptance/dc/peers/sorting.feature.

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

module('Acceptance | dc / peers / sorting', function (hooks) {
  setupAcceptanceTest(hooks);

  nspaceScenario(
    'Scenario: Sorting Peers from the toolbar dropdown',
    async function (assert, nspace) {
      createModels('dc', 1, 'dc-1');
      createModels('peer', 3, [{ Name: 'z-peer' }, { Name: 'b-peer' }, { Name: 'a-peer' }]);

      await visit('peers', { dc: 'dc-1' }, { nspace });
      assert.equal(currentURL(), nspaceURL(nspace, '/dc-1/peers'));
      assertVisibleCount(assert, page().peers, 3, 'peer models');

      await page().sort.selected();
      await page().sort.options.objectAt(1).button();
      assertPropertiesVertically(
        assert,
        page().peers,
        'name',
        ['z-peer', 'b-peer', 'a-peer'],
        'peers'
      );

      await page().sort.selected();
      await page().sort.options.objectAt(0).button();
      assertPropertiesVertically(
        assert,
        page().peers,
        'name',
        ['a-peer', 'b-peer', 'z-peer'],
        'peers'
      );
    }
  );
});
