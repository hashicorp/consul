/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

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
} from 'consul-ui/tests/helpers/acceptance';

const serf = (Status, Output) => ({
  Type: '',
  Name: 'Serf Health Status',
  CheckID: 'serfHealth',
  Status,
  Output,
});
const nodeCheck = {
  Type: '',
  Name: 'Node Health Check',
  CheckID: 'serfHealth',
  ServiceID: '',
  Status: 'passing',
  Output: 'Agent alive and reachable',
};

const visitInstance = async function (assert, nspace, id) {
  await visit(
    'instance',
    { dc: 'dc1', service: 'service-0', node: 'another-node', id },
    { nspace }
  );
  assert.equal(
    currentURL(),
    nspaceURL(nspace, `/dc1/services/service-0/instances/another-node/${id}/health-checks`)
  );
  assert.ok(page().tabs.healthChecksIsSelected, 'health checks is selected');
};

const withName = (name) =>
  page().healthChecks.filter((item) => item.isVisible && item.name === name).length;

module('Acceptance | dc / services / instances / health-checks', function (hooks) {
  setupAcceptanceTest(hooks);

  // Background
  hooks.beforeEach(function () {
    createModels('dc', 1, 'dc1');
    createModels('proxy', 1, [
      { ServiceProxy: { DestinationServiceName: 'service-1', DestinationServiceID: null } },
    ]);
  });

  nspaceScenario('Scenario: A failing serf check', async function (assert, nspace) {
    createModels('instance', 2, [
      { Service: { ID: 'service-0-with-id' }, Node: { Node: 'node-0' } },
      {
        Service: { ID: 'service-1-with-id' },
        Node: { Node: 'another-node' },
        Checks: [serf('critical', 'ouch')],
      },
    ]);

    await visitInstance(assert, nspace, 'service-1-with-id');
    assert.ok(page().tabs.healthChecksTab.criticalSerfNotice, 'shows the serf notice');
  });

  nspaceScenario('Scenario: A passing serf check', async function (assert, nspace) {
    createModels('instance', 2, [
      { Service: { ID: 'service-0-with-id' }, Node: { Node: 'node-0' } },
      {
        Service: { ID: 'service-1-with-id' },
        Node: { Node: 'another-node' },
        Checks: [serf('passing', 'Agent alive and reachable')],
      },
    ]);

    await visitInstance(assert, nspace, 'service-1-with-id');
    assert.notOk(page().tabs.healthChecksTab.criticalSerfNotice, 'no serf notice');
  });

  nspaceScenario(
    'Scenario: Node health check should be hidden on agentless service instances',
    async function (assert, nspace) {
      createModels('instance', 1, [
        {
          Service: { ID: 'service-0-with-id' },
          Node: { Node: 'another-node', Meta: { 'synthetic-node': true } },
          Checks: [nodeCheck, serf('critical', 'ouch')],
        },
      ]);

      await visitInstance(assert, nspace, 'service-0-with-id');
      assertVisibleCount(assert, page().tabs.healthChecksTab.healthChecks, 1, 'healthCheck models');
      assert.equal(withName('Serf Health Status'), 1, 'sees the serf check');
    }
  );

  nspaceScenario(
    'Scenario: Node health checks should be visible on non-agentless service instances',
    async function (assert, nspace) {
      createModels('instance', 1, [
        {
          Service: { ID: 'service-0-with-id' },
          Node: { Node: 'another-node' },
          Checks: [nodeCheck, serf('critical', 'ouch')],
        },
      ]);

      await visitInstance(assert, nspace, 'service-0-with-id');
      assertVisibleCount(assert, page().tabs.healthChecksTab.healthChecks, 2, 'healthCheck models');
      assert.equal(withName('Serf Health Status'), 1, 'sees the serf check');
      assert.equal(withName('Node Health Check'), 1, 'sees the node check');
    }
  );
});
