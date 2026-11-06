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
      api.server.createList('dc', 1, 'dc-1');
      // Use the default fixture data — no override needed.
      // The 'synthetic-node' field defaults to false in the fixture, so all
      // nodes pass the (reject-by 'Meta.synthetic-node' items) filter on the page.
      api.server.createList('node', 3);

      await visit('nodes', { dc: 'dc-1' }, { nspace });

      assert.equal(currentURL(), nspaceURL(nspace, '/dc-1/nodes'));
      assert.equal(document.title, 'Nodes - Consul');
      assertVisibleCount(assert, page().nodes, 3, 'nodes');

      // The api-double marks the first node as peered, and the breadcrumb link
      // keeps that peer segment (/:billing/dc-1/nodes). Use an unpeered node so
      // "back" can be asserted exactly.
      await page().nodes.objectAt(1).node();

      await click('[data-test-breadcrumb-item]:first-child a');

      assert.equal(currentURL(), nspaceURL(nspace, '/dc-1/nodes'));
    }
  );
});
