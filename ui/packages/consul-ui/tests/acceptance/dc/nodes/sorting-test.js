/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module } from 'qunit';

import {
  setupAcceptanceTest,
  nspaceScenario,
  createModels,
  visit,
  page,
  assertPropertiesVertically,
} from 'consul-ui/tests/helpers/acceptance';

const notSynthetic = { 'synthetic-node': false };

module('Acceptance | dc / nodes / sorting', function (hooks) {
  setupAcceptanceTest(hooks);

  nspaceScenario('Scenario: Sorting the node list by name', async function (assert, nspace) {
    createModels('dc', 1, 'dc-1');
    createModels('node', 4, [
      { Node: 'Node-B', Meta: notSynthetic },
      { Node: 'Node-D', Meta: notSynthetic },
      { Node: 'Node-A', Meta: notSynthetic },
      { Node: 'Node-C', Meta: notSynthetic },
    ]);

    await visit('nodes', { dc: 'dc-1' }, { nspace });
    // ascending (A-Z)
    await page().sort.name();
    assertPropertiesVertically(
      assert,
      page().nodes,
      'name',
      ['Node-A', 'Node-B', 'Node-C', 'Node-D'],
      'nodes'
    );
    // descending (Z-A)
    await page().sort.name();
    assertPropertiesVertically(
      assert,
      page().nodes,
      'name',
      ['Node-D', 'Node-C', 'Node-B', 'Node-A'],
      'nodes'
    );
  });

  nspaceScenario('Scenario: Sorting the node list by health', async function (assert, nspace) {
    createModels('dc', 1, 'dc-1');
    createModels('node', 3, [
      { Node: 'Node-passing', Meta: notSynthetic, Checks: [{ Status: 'passing', ServiceID: '' }] },
      {
        Node: 'Node-critical',
        Meta: notSynthetic,
        Checks: [{ Status: 'critical', ServiceID: '' }],
      },
      { Node: 'Node-warning', Meta: notSynthetic, Checks: [{ Status: 'warning', ServiceID: '' }] },
    ]);

    await visit('nodes', { dc: 'dc-1' }, { nspace });
    // ascending (unhealthy first)
    await page().sort.health();
    assertPropertiesVertically(
      assert,
      page().nodes,
      'name',
      ['Node-critical', 'Node-warning', 'Node-passing'],
      'nodes'
    );
    // descending (healthy first)
    await page().sort.health();
    assertPropertiesVertically(
      assert,
      page().nodes,
      'name',
      ['Node-passing', 'Node-warning', 'Node-critical'],
      'nodes'
    );
  });
});
