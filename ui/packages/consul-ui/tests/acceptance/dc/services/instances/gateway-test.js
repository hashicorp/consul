/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

// Native QUnit port of tests/acceptance/dc/services/instances/gateway.feature.

import { module } from 'qunit';

import {
  setupAcceptanceTest,
  nspaceScenario,
  nspaceURL,
  createModels,
  visit,
  page,
  currentURL,
  assertPropertiesVertically,
} from 'consul-ui/tests/helpers/acceptance';

module(
  'Acceptance | dc / services / instances / gateway: Show Gateway Service Instance',
  function (hooks) {
    setupAcceptanceTest(hooks);

    nspaceScenario('Scenario: A Gateway Service instance', async function (assert, nspace) {
      createModels('dc', 1, 'dc1');
      createModels('proxy', 1, [
        { ServiceProxy: { DestinationServiceName: 'service-1', DestinationServiceID: null } },
      ]);
      createModels('instance', 1, [
        {
          Service: {
            Kind: 'mesh-gateway',
            Name: 'gateway',
            ID: 'gateway-with-id',
            TaggedAddresses: {
              lan: { Address: '127.0.0.1', Port: 8080 },
              wan: { Address: '92.68.0.0', Port: 8081 },
            },
          },
        },
      ]);

      await visit(
        'instance',
        { dc: 'dc1', service: 'gateway', node: 'node-0', id: 'gateway-with-id' },
        { nspace }
      );
      assert.equal(
        currentURL(),
        nspaceURL(nspace, '/dc1/services/gateway/instances/node-0/gateway-with-id/health-checks')
      );
      const { tabs } = page();
      assert.ok(tabs.healthChecksIsSelected, 'health checks is selected');

      await tabs.addresses();
      assert.ok(tabs.addressesIsSelected, 'addresses is selected');
      assert.equal(page().addresses.length, 2, 'two addresses');
      assertPropertiesVertically(
        assert,
        page().addresses,
        'address',
        ['127.0.0.1:8080', '92.68.0.0:8081'],
        'addresses'
      );
    });
  }
);
