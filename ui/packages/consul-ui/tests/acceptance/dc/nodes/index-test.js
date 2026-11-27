/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module } from 'qunit';
import { fillIn } from '@ember/test-helpers';

import {
  setupAcceptanceTest,
  nspaceScenario,
  nspaceURL,
  nspaceEndpoint,
  createModels,
  respondWith,
  visit,
  page,
  currentURL,
  assertVisibleCount,
  requestMade,
} from 'consul-ui/tests/helpers/acceptance';

const notSynthetic = { 'synthetic-node': false };

// Background: dc-1 with a leader at 211.245.86.75.
const setup = function () {
  createModels('dc', 1, 'dc-1');
  respondWith('/v1/status/leader', { body: '"211.245.86.75:8500"\n' });
};

const visitNodes = async function (assert, nspace) {
  await visit('nodes', { dc: 'dc-1' }, { nspace });
  assert.equal(currentURL(), nspaceURL(nspace, '/dc-1/nodes'));
};

module('Acceptance | dc / nodes / index', function (hooks) {
  setupAcceptanceTest(hooks);

  hooks.beforeEach(setup);

  nspaceScenario(
    'Scenario: Viewing a node with an unhealthy NodeCheck',
    async function (assert, nspace) {
      createModels('node', 1, [{ Checks: [{ Status: 'critical', ServiceID: '' }] }]);

      await visitNodes(assert, nspace);
      assertVisibleCount(assert, page().nodes, 1, 'node models');
      assert.strictEqual(page().nodes.objectAt(0).status, 'critical');
    }
  );

  nspaceScenario(
    'Scenario: Viewing nodes list page should not show synthetic nodes',
    async function (assert, nspace) {
      createModels('node', 3, [
        { Meta: { 'synthetic-node': true }, Checks: [{ Status: 'passing', ServiceID: '' }] },
        { Meta: notSynthetic, Checks: [{ Status: 'passing', ServiceID: '' }] },
        { Meta: notSynthetic, Checks: [{ Status: 'critical', ServiceID: '' }] },
      ]);

      await visitNodes(assert, nspace);
      assertVisibleCount(assert, page().nodes, 2, 'node models');
    }
  );

  nspaceScenario(
    'Scenario: Viewing a node with an unhealthy ServiceCheck',
    async function (assert, nspace) {
      createModels('node', 1, [
        {
          Checks: [
            { Status: 'passing', ServiceID: '' },
            { Status: 'critical', ServiceID: 'web' },
          ],
          Meta: notSynthetic,
        },
      ]);

      await visitNodes(assert, nspace);
      assertVisibleCount(assert, page().nodes, 1, 'node models');
      assert.strictEqual(page().nodes.objectAt(0).status, 'passing');
    }
  );

  nspaceScenario('Scenario: Viewing nodes in the listing', async function (assert, nspace) {
    createModels('node', 3, [
      { Meta: notSynthetic, Checks: [{ Status: 'passing', ServiceID: '' }] },
      { Meta: notSynthetic, Checks: [{ Status: 'passing', ServiceID: '' }] },
      { Meta: notSynthetic, Checks: [{ Status: 'critical', ServiceID: '' }] },
    ]);

    await visitNodes(assert, nspace);
    assert.equal(document.title, 'Nodes - Consul');
    requestMade(
      assert,
      'GET',
      nspaceEndpoint('/v1/internal/ui/nodes?dc=dc-1&ns=@namespace', nspace)
    );
    assertVisibleCount(assert, page().nodes, 3, 'node models');
  });

  nspaceScenario('Scenario: Seeing the leader in node listing', async function (assert, nspace) {
    createModels('node', 3, [
      {
        Address: '211.245.86.75',
        Checks: [{ Status: 'critical', Name: 'Warning check' }],
        Meta: notSynthetic,
      },
      { Address: '10.0.0.1', Checks: [{ Status: 'passing' }], Meta: notSynthetic },
      { Address: '10.0.0.3', Checks: [{ Status: 'passing' }], Meta: notSynthetic },
    ]);

    await visitNodes(assert, nspace);
    assertVisibleCount(assert, page().nodes, 3, 'node models');
    assert.ok(page().nodes.objectAt(0).leader, 'the first node is the leader');
  });

  nspaceScenario(
    'Scenario: Searching the nodes with name and IP address',
    async function (assert, nspace) {
      createModels('node', 3, [
        { Node: 'node-01', Address: '10.0.0.0', Meta: notSynthetic },
        { Node: 'node-02', Address: '10.0.0.1', Meta: notSynthetic },
        { Node: 'node-03', Address: '10.0.0.2', Meta: notSynthetic },
      ]);

      await visit('nodes', { dc: 'dc-1' }, { nspace });
      assertVisibleCount(assert, page().nodes, 3, 'node models');

      const named = (name) =>
        page().nodes.filter((item) => item.isVisible && item.name === name).length;

      await fillIn('[name="search"]', 'node-01');
      assertVisibleCount(assert, page().nodes, 1, 'node models');
      assert.equal(named('node-01'), 1, 'sees node-01');

      await fillIn('[name="search"]', '10.0.0.1');
      assertVisibleCount(assert, page().nodes, 1, 'node models');
      assert.equal(named('node-02'), 1, 'sees node-02');
    }
  );

  nspaceScenario(
    'Scenario: Viewing an empty nodes page with acl enabled',
    async function (assert, nspace) {
      createModels('dc', 1, 'dc-1');
      createModels('node', 0);

      await visitNodes(assert, nspace);
      assert.equal(document.title, 'Nodes - Consul');
      assertVisibleCount(assert, page().nodes, 0, 'node models');
      assert
        .dom('.empty-state p')
        .includesText(
          "There don't seem to be any registered Nodes in this Consul cluster, or you may not have service:read and node:read permissions access to this view."
        );
      assert.dom('[data-test-empty-state-login]').exists();
    }
  );
});
