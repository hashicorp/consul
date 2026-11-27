/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module } from 'qunit';
import { click } from '@ember/test-helpers';

import {
  setupAcceptanceTest,
  nspaceScenario,
  nspaceURL,
  nspaceYAML,
  createModels,
  visit,
  page,
  currentURL,
} from 'consul-ui/tests/helpers/acceptance';

// Background: service-0 with a downstream from routing config.
const visitTopology = async function (assert, nspace) {
  createModels('dc', 1, 'dc1');
  createModels('node', 1);
  createModels('service', 1, [{ Service: { Name: 'service-0', ID: 'service-0-with-id' } }]);
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
    Source: routing-config
`,
      nspace
    )
  );

  await visit('service', { dc: 'dc1', service: 'service-0' }, { nspace });
  assert.ok(page().tabs.topologyIsVisible, 'the topology tab is shown');
};

module('Acceptance | dc / services / show / topology / routing-config', function (hooks) {
  setupAcceptanceTest(hooks);

  nspaceScenario(
    'Scenario: Given the Source is routing config, show Source Type',
    async function (assert, nspace) {
      await visitTopology(assert, nspace);
      assert.dom('[data-test-topology-metrics-source-type]').includesText('Routing configuration');
    }
  );

  nspaceScenario(
    'Scenario: Given the Source is routing config, redirect to Routing Config page',
    async function (assert, nspace) {
      await visitTopology(assert, nspace);
      await click('[data-test-topology-metrics-card]');
      assert.equal(currentURL(), nspaceURL(nspace, '/dc1/routing-config/db'));
    }
  );
});
