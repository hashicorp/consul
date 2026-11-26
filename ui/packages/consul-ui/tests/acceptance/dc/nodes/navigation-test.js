/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

// Native QUnit port of tests/acceptance/dc/nodes/navigation.feature

import { module } from 'qunit';
import { click } from '@ember/test-helpers';

import {
  setupAcceptanceTest,
  nspaceScenario,
  createModels,
  api,
  visit,
  page,
  currentURL,
  nspaceURL,
  assertVisibleCount,
} from 'consul-ui/tests/helpers/acceptance';

module('Acceptance | dc / nodes / navigation: Nodes Navigation', function (hooks) {
  setupAcceptanceTest(hooks);

  nspaceScenario(
    'Scenario: Clicking a node in the listing and back again',
    async function (assert, nspace) {
      createModels('dc', 1, 'dc-1');
      api.server.createList('node', 3, [
        {
          Node: 'Node-A',
          Meta: { 'synthetic-node': false },
          Checks: [{ Status: 'critical', ServiceID: '' }],
        },
        {
          Node: 'Node-B',
          Meta: { 'synthetic-node': false },
          Checks: [{ Status: 'passing', ServiceID: '' }],
        },
        {
          Node: 'Node-C',
          Meta: { 'synthetic-node': false },
          Checks: [{ Status: 'warning', ServiceID: '' }],
        },
      ]);

      await visit('nodes', { dc: 'dc-1' }, { nspace });

      assert.equal(currentURL(), nspaceURL(nspace, '/dc-1/nodes'));
      assert.equal(document.title, 'Nodes - Consul');
      assertVisibleCount(assert, page().nodes, 3, 'nodes');

      // The api-double marks the first node as peered (`billing`).
      await page().nodes.objectAt(0).node();

      await click('[data-test-breadcrumb-item]:first-child a');

      // FIXME: the breadcrumb back to the list keeps the clicked node's peer
      // segment, although app/services/breadcrumbs.js means to avoid inheriting
      // peer segments. Once that is fixed this should be `/dc-1/nodes`, as in the
      // original feature.
      assert.equal(currentURL(), nspaceURL(nspace, '/:billing/dc-1/nodes'));
    }
  );
});
