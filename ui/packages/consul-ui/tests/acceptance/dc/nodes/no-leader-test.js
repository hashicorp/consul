/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

// Native QUnit port of tests/acceptance/dc/nodes/no-leader.feature.

import { module } from 'qunit';

import {
  setupAcceptanceTest,
  nspaceScenario,
  nspaceURL,
  createModels,
  respondWith,
  visit,
  page,
  currentURL,
  assertVisibleCount,
} from 'consul-ui/tests/helpers/acceptance';

module('Acceptance | dc / nodes / no-leader', function (hooks) {
  setupAcceptanceTest(hooks);

  nspaceScenario("Scenario: Leader hasn't been elected", async function (assert, nspace) {
    const notSynthetic = { 'synthetic-node': false };
    createModels('dc', 1, 'dc-1');
    createModels('node', 3, [
      { Node: 'Node-A', Meta: notSynthetic, Checks: [{ Status: 'critical', ServiceID: '' }] },
      { Node: 'Node-B', Meta: notSynthetic, Checks: [{ Status: 'passing', ServiceID: '' }] },
      { Node: 'Node-C', Meta: notSynthetic, Checks: [{ Status: 'warning', ServiceID: '' }] },
    ]);
    respondWith('/v1/status/leader', { body: '""\n' });

    await visit('nodes', { dc: 'dc-1' }, { nspace });
    assert.equal(currentURL(), nspaceURL(nspace, '/dc-1/nodes'));
    assertVisibleCount(assert, page().nodes, 3, 'node models');
    assert.dom('.consul-node-table tbody tr [data-test-leader]').doesNotExist('no leader shown');
  });
});
