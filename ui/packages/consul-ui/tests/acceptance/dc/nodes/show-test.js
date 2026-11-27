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
  respondWith,
  setSettings,
  setLatency,
  visit,
  page,
  currentURL,
} from 'consul-ui/tests/helpers/acceptance';

// A page-object property that may not exist.
const isShown = function (get) {
  try {
    return !!get();
  } catch (e) {
    return false;
  }
};

module('Acceptance | dc / nodes / show: Show node', function (hooks) {
  setupAcceptanceTest(hooks);

  hooks.beforeEach(function () {
    createModels('dc', 1, 'dc1');
  });

  // 2 nodes are required for the RTT tab to be visible
  nspaceScenario(
    'Scenario: Given 2 nodes all the tabs are visible and clickable',
    async function (assert, nspace) {
      createModels('node', 2);

      await visit('node', { dc: 'dc1', node: 'node-0' }, { nspace });
      const { tabs } = page();
      assert.ok(tabs.healthChecksIsSelected, 'health checks is selected');

      await tabs.serviceInstances();
      assert.ok(tabs.serviceInstancesIsSelected, 'service instances is selected');
      await tabs.roundTripTime();
      assert.ok(tabs.roundTripTimeIsSelected, 'round trip time is selected');
      await tabs.lockSessions();
      assert.ok(tabs.lockSessionsIsSelected, 'lock sessions is selected');
      await tabs.metadata();
      assert.ok(tabs.metadataIsSelected, 'metadata is selected');
    }
  );

  nspaceScenario(
    'Scenario: Given 1 node with no checks all the tabs are visible but the serviceInstances tab is selected',
    async function (assert, nspace) {
      createModels('node', 1, { ID: 'node-0', Checks: [] });

      await visit('node', { dc: 'dc1', node: 'node-0' }, { nspace });
      const { tabs } = page();
      assert.ok(tabs.serviceInstancesIsSelected, 'service instances is selected');
      assert.ok(
        isShown(() => tabs.healthChecksIsVisible),
        'health checks is visible'
      );
      assert.ok(
        isShown(() => tabs.serviceInstancesIsVisible),
        'service instances is visible'
      );
      assert
        .dom('[data-test-tab="tab_round-trip-time"]')
        .doesNotExist('round trip time is not shown');
      assert.ok(
        isShown(() => tabs.lockSessionsIsVisible),
        'lock sessions is visible'
      );
    }
  );

  nspaceScenario(
    'Scenario: A node warns when deregistered whilst blocking',
    async function (assert, nspace) {
      createModels('node', 1, { ID: 'node-0' });
      setSettings({ 'consul:client': { blocking: 1, throttle: 200 } });
      setLatency(100);

      await visit('node', { dc: 'dc1', node: 'node-0' }, { nspace });
      assert.equal(currentURL(), nspaceURL(nspace, '/dc1/nodes/node-0/health-checks'));
      assert.equal(document.title, 'node-0 - Consul');

      respondWith('/v1/internal/ui/node/node-0', { status: 404 });
      const noLongerExists = () => {
        const notification = document.querySelector('[data-notification]');
        return notification && notification.textContent.includes('no longer exists');
      };
      try {
        await waitUntil(noLongerExists, { timeout: 5000 });
      } catch (e) {
        // reported by the assertion below
      }
      assert.dom('[data-notification]').includesText('no longer exists');
    }
  );
});
