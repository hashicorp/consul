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
  respondWith,
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

module(
  'Acceptance | dc / services / show / topology / index: Show Topology tab for Service',
  function (hooks) {
    setupAcceptanceTest(hooks);

    hooks.beforeEach(function () {
      createModels('dc', 1, 'dc1');
      createModels('node', 1);
      createModels('service', 1, [{ Service: { Name: 'service-0', ID: 'service-0-with-id' } }]);
    });

    nspaceScenario(
      'Scenario: Given a service, the Topology tab should display',
      async function (assert, nspace) {
        await visit('service', { dc: 'dc1', service: 'service-0' }, { nspace });
        assert.ok(page().tabs.topologyIsVisible, 'the topology tab is shown');
        assert.equal(currentURL(), nspaceURL(nspace, '/dc1/services/service-0/topology'));
      }
    );

    nspaceScenario(
      'Scenario: Given connect is disabled, the Topology tab should not display or error',
      async function (assert, nspace) {
        respondWith('/v1/discovery-chain/service-0', {
          status: 500,
          body: 'Connect must be enabled in order to use this endpoint',
        });

        await visit('service', { dc: 'dc1', service: 'service-0' }, { nspace });
        assert.notOk(
          isShown(() => page().tabs.topologyIsVisible),
          'the topology tab is not shown'
        );
        assert.equal(currentURL(), nspaceURL(nspace, '/dc1/services/service-0/instances'));
      }
    );
  }
);
