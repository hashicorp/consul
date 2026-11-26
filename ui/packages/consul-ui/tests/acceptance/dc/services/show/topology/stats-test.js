/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

// Native QUnit port of tests/acceptance/dc/services/show/topology/stats.feature.

import { module } from 'qunit';

import {
  setupAcceptanceTest,
  nspaceScenario,
  nspaceYAML,
  createModels,
  setMetricsProvider,
  visit,
  page,
} from 'consul-ui/tests/helpers/acceptance';

const visitService = async function (assert, nspace, service) {
  await visit('service', { dc: 'dc1', service }, { nspace });
  assert.ok(page().tabs.topologyIsVisible, 'the topology tab is shown');
};

const service0 = [{ Service: { Name: 'service-0', ID: 'service-0-with-id' } }];

module('Acceptance | dc / services / show / topology / stats', function (hooks) {
  setupAcceptanceTest(hooks);

  hooks.beforeEach(function () {
    createModels('dc', 1, 'dc1');
  });

  nspaceScenario(
    'Scenario: Given metrics is disabled, the Topology tab should not display metrics',
    async function (assert, nspace) {
      createModels('node', 1);
      createModels('service', 1, service0);

      await visitService(assert, nspace, 'service-0');
      assert.dom('[data-test-topology-metrics-stats]').doesNotExist('no metrics stats');
    }
  );

  nspaceScenario(
    'Scenario: Given metrics is enabled, the Topology tab should display metrics',
    async function (assert, nspace) {
      setMetricsProvider('prometheus');
      createModels('node', 1);
      createModels('service', 1, service0);

      await visitService(assert, nspace, 'service-0');
      assert.dom('[data-test-topology-metrics-stats]').exists('shows metrics stats');
    }
  );

  nspaceScenario(
    'Scenario: Given metrics is enabled, metrics stats are disabled for an ingress gateway Topology',
    async function (assert, nspace) {
      setMetricsProvider('prometheus');
      createModels('node', 1);
      createModels('service', 1, [
        {
          Service: {
            Name: 'ingress-gateway',
            Kind: 'ingress-gateway',
            ID: 'ingress-gateway-with-id',
          },
        },
      ]);

      await visitService(assert, nspace, 'ingress-gateway');
      assert.dom('[data-test-topology-metrics-stats]').doesNotExist('no metrics stats');
      assert.dom('[data-test-topology-metrics-status]').exists('shows metrics status');
    }
  );

  nspaceScenario(
    'Scenario: Given metrics is enabled, metric stats are disabled for ingress gateway as downstream services',
    async function (assert, nspace) {
      setMetricsProvider('prometheus');
      createModels('node', 1);
      createModels('service', 1, service0);
      createModels(
        'topology',
        1,
        nspaceYAML(
          `
Upstreams: []
Downstreams:
  - Name: db
    Namespace: @namespace
    Datacenter: dc1
    Intention: {}
    Kind: ingress-gateway
`,
          nspace
        )
      );

      await visitService(assert, nspace, 'service-0');
      assert.dom('[data-test-sparkline]').exists('shows a sparkline');
      assert
        .dom('[data-test-topology-metrics-downstream-stats]')
        .doesNotExist('no downstream stats');
    }
  );
});
