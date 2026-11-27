/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module } from 'qunit';

import {
  setupAcceptanceTest,
  nspaceScenario,
  createModels,
  setLocalDatacenter,
  setUiConfig,
  visit,
} from 'consul-ui/tests/helpers/acceptance';

const prometheus = { metrics_proxy_enabled: true, metrics_provider: 'prometheus' };

const visitWeb = function (nspace) {
  return visit('service', { dc: 'datacenter', service: 'web' }, { nspace });
};

module('Acceptance | dc / services / show / topology / metrics', function (hooks) {
  setupAcceptanceTest(hooks);

  // Background: web has an upstream db with an intention.
  hooks.beforeEach(function () {
    createModels('dc', 1, 'datacenter');
    setLocalDatacenter('datacenter');
    createModels('intention', 1, {
      SourceNS: 'default',
      SourceName: 'web',
      DestinationNS: 'default',
      DestinationName: 'db',
      ID: 'intention-id',
    });
    createModels('node', 1);
    createModels('service', 1, [{ Service: { Name: 'web', Kind: null } }]);
    createModels('topology', 1, {
      Downstreams: [],
      Upstreams: [{ Name: 'db', Namespace: 'default', Datacenter: 'datacenter', Intention: {} }],
    });
  });

  nspaceScenario(
    'Scenario: Metrics is not enabled with prometheus provider',
    async function (assert, nspace) {
      await visitWeb(nspace);
      assert.dom('[data-test-sparkline]').doesNotExist('no sparkline');
    }
  );

  nspaceScenario(
    'Scenario: Metrics is enabled with prometheus provider',
    async function (assert, nspace) {
      createModels('dc', 1, 'datacenter');
      setLocalDatacenter('datacenter');
      setUiConfig(prometheus);

      await visitWeb(nspace);
      assert.dom('[data-test-sparkline]').exists('shows a sparkline');
    }
  );

  nspaceScenario(
    'Scenario: Metrics enabled but service source is Consul API Gateway',
    async function (assert, nspace) {
      createModels('dc', 1, 'datacenter');
      setLocalDatacenter('datacenter');
      // The original fixture YAML was mis-indented; this is the data it parsed to.
      createModels('service', 1, [
        {
          Service: null,
          Name: 'web',
          Kind: null,
          Meta: null,
          'external-source': 'consul-api-gateway',
        },
      ]);
      setUiConfig(prometheus);

      await visitWeb(nspace);
      assert.dom('[data-test-sparkline]').doesNotExist('no sparkline');
    }
  );
});
