/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

// Native QUnit port of tests/acceptance/dc/nodes/empty-ids.feature.

import { module } from 'qunit';

import {
  setupAcceptanceTest,
  nspaceScenario,
  nspaceURL,
  createModels,
  visit,
  page,
  currentURL,
  assertPropertiesVertically,
} from 'consul-ui/tests/helpers/acceptance';

module(
  'Acceptance | dc / nodes / empty-ids: Hedge for if nodes come in over the API with no ID',
  function (hooks) {
    setupAcceptanceTest(hooks);

    nspaceScenario('Scenario: A node list with some missing IDs', async function (assert, nspace) {
      const names = ['name-1', 'name-2', 'name-3', 'name-4', 'name-5'];
      createModels('dc', 1, 'dc-1');
      createModels(
        'node',
        5,
        names.map((Node, i) => ({
          ID: i === 0 ? 'id-1' : '',
          Node,
          Meta: { 'synthetic-node': false },
          Checks: [{ Status: 'passing' }],
        }))
      );

      await visit('nodes', { dc: 'dc-1' }, { nspace });
      assert.equal(currentURL(), nspaceURL(nspace, '/dc-1/nodes'));
      assertPropertiesVertically(assert, page().nodes, 'name', names, 'nodes');
    });
  }
);
