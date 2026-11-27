/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module } from 'qunit';

import {
  setupAcceptanceTest,
  nspaceScenario,
  nspaceEndpoint,
  createModels,
  respondWith,
  visit,
  page,
  requestNotMade,
} from 'consul-ui/tests/helpers/acceptance';

module('Acceptance | dc / services / show-routing: Show Routing for Service', function (hooks) {
  setupAcceptanceTest(hooks);

  nspaceScenario(
    'Scenario: Given a service, the Routing tab should display',
    async function (assert, nspace) {
      createModels('dc', 1, 'dc1');
      createModels('node', 1);
      createModels('service', 1, [{ Service: { Name: 'service-0', ID: 'service-0-with-id' } }]);

      await visit('service', { dc: 'dc1', service: 'service-0' }, { nspace });
      assert.equal(document.title, 'service-0 - Consul');
      assert.ok(page().tabs.routingIsVisible, 'shows the routing tab');
    }
  );

  nspaceScenario(
    'Scenario: Given connect is disabled, the Routing tab should not display or error',
    async function (assert, nspace) {
      createModels('dc', 2, ['dc1', 'dc2']);
      createModels('node', 1);
      createModels('service', 2, [
        { Service: { Name: 'service-0', ID: 'service-0-with-id' } },
        { Service: { Name: 'service-1', ID: 'service-1-with-id' } },
      ]);
      respondWith('/v1/discovery-chain/service-0', {
        status: 500,
        body: 'Connect must be enabled in order to use this endpoint',
      });

      await visit('service', { dc: 'dc1', service: 'service-0' }, { nspace });
      assert.dom('[data-test-tab="tab_routing"]').doesNotExist('no routing tab');
      assert.dom('[data-test-error]').doesNotExist('no error');

      // Not entirely sure if having one dc not having connect and another
      // having connect ever actually happen
      await visit('service', { dc: 'dc2', service: 'service-1' }, { nspace });
      assert.ok(page().tabs.routingIsVisible, 'shows the routing tab in dc2');

      await visit('service', { dc: 'dc1', service: 'service-0' }, { nspace });
      requestNotMade(
        assert,
        'GET',
        nspaceEndpoint('/v1/discovery-chain/service-0?dc=dc1&ns=@namespace', nspace)
      );
      assert.dom('[data-test-tab="tab_routing"]').doesNotExist('no routing tab');
      assert.dom('[data-test-error]').doesNotExist('no error');
    }
  );
});
