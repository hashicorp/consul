/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module } from 'qunit';
import { waitUntil } from '@ember/test-helpers';

import {
  setupAcceptanceTest,
  nspaceScenario,
  nspaceURL,
  createModels,
  setSettings,
  setLatency,
  visit,
  page,
  currentURL,
  assertVisibleCount,
} from 'consul-ui/tests/helpers/acceptance';

const check = (Name, ServiceID, Status, extra = {}) => ({
  Name,
  ServiceID,
  Output: 'Output of check',
  Status,
  ...extra,
});

const visitInstance = async function (assert, nspace, service, node, id) {
  await visit('instance', { dc: 'dc1', service, node, id }, { nspace });
  assert.equal(
    currentURL(),
    nspaceURL(nspace, `/dc1/services/${service}/instances/${node}/${id}/health-checks`)
  );
};

module('Acceptance | dc / services / instances / show: Show Service Instance', function (hooks) {
  setupAcceptanceTest(hooks);

  // Background
  hooks.beforeEach(function () {
    createModels('dc', 1, 'dc1');
    createModels('instance', 3, [
      {
        Service: { ID: 'service-0-with-id', Meta: { 'external-source': 'consul' } },
        Node: { Node: 'node-0' },
      },
      {
        Service: {
          ID: 'service-1-with-id',
          Tags: ['Tag1', 'Tag2'],
          Meta: {
            'consul-dashboard-url': 'http://url.com',
            'external-source': 'nomad',
            'test-meta': 'test-meta-value',
          },
        },
        Node: { Node: 'another-node' },
        Checks: [
          check('Service check', 'service-0', 'passing'),
          check('Service check', 'service-0', 'warning'),
          check('Service check', 'service-0', 'critical', { Type: 'http' }),
          check('Node check', '', 'passing'),
          check('Node check', '', 'warning'),
          check('Node check', '', 'critical'),
        ],
      },
      {
        Service: {
          ID: 'service-2-with-id',
          Meta: { 'external-source': 'kubernetes', 'synthetic-node': true },
        },
        Node: { Node: 'node-2', Meta: { 'synthetic-node': true } },
      },
    ]);
  });

  nspaceScenario('Scenario: A Service instance has no Proxy', async function (assert, nspace) {
    createModels('proxy', 1, [
      { ServiceProxy: { DestinationServiceName: 'service-1', DestinationServiceID: null } },
    ]);

    await visitInstance(assert, nspace, 'service-0', 'another-node', 'service-1-with-id');
    assert.strictEqual(page().externalSource, 'nomad');
    assert.dom('[data-test-service-instance-node-name]').includesText('another-node');

    const { tabs } = page();
    assert.dom('[data-test-tab="tab_upstreams"]').doesNotExist('no upstreams tab');
    assert.ok(tabs.healthChecksIsSelected, 'health checks is selected');
    assertVisibleCount(assert, page().healthChecks, 6, 'healthCheck models');

    await tabs['tags&Metadata']();
    assert.ok(tabs['tags&MetadataIsSelected'], 'tags & metadata is selected');
    assert.dom('[data-test-tags] span:nth-child(1)').includesText('Tag1');
    assert.dom('[data-test-tags] span:nth-child(2)').includesText('Tag2');
    assert.equal(page().metadata.length, 3, 'three metadata entries');
    assert.equal(document.title, 'service-1-with-id - Consul');
  });

  nspaceScenario(
    'Scenario: A Service instance warns when deregistered whilst blocking',
    async function (assert, nspace) {
      createModels('proxy', 1, [
        { ServiceProxy: { DestinationServiceName: 'service-1', DestinationServiceID: null } },
      ]);
      setSettings({ 'consul:client': { blocking: 1, throttle: 200 } });
      setLatency(100);

      await visitInstance(assert, nspace, 'service-0', 'node-0', 'service-0-with-id');
      createModels('instance', 0);
      const deregistered = () => {
        const notification = document.querySelector('[data-notification]');
        return notification && notification.textContent.includes('deregistered');
      };
      try {
        await waitUntil(deregistered, { timeout: 5000 });
      } catch (e) {
        // reported by the assertion below
      }
      assert.dom('[data-notification]').includesText('deregistered');
    }
  );

  nspaceScenario(
    'Scenario: A Service instance without a Proxy does not display Proxy Info tab',
    async function (assert, nspace) {
      createModels('proxy', 1, [
        { ServiceProxy: { DestinationServiceName: 'service-1', DestinationServiceID: null } },
      ]);

      await visitInstance(assert, nspace, 'service-0', 'node-0', 'service-0-with-id');
      assert.dom('[data-test-tab="tab_proxy"]').doesNotExist('no proxy tab');
    }
  );

  nspaceScenario(
    'Scenario: A Service instance with a synthetic node does not display the node name',
    async function (assert, nspace) {
      await visitInstance(assert, nspace, 'service-2', 'node-2', 'service-2-with-id');
      assert.strictEqual(page().externalSource, 'kubernetes');
      const nodeName = document.querySelector('[data-test-service-instance-node-name]');
      assert.notOk(
        nodeName && nodeName.textContent.includes('node-2'),
        'the node name is not shown'
      );
    }
  );
});
