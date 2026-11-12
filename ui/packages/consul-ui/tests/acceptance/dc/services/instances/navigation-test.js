/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

// Native QUnit port of tests/acceptance/dc/services/instances/navigation.feature

import { module } from 'qunit';
import {
  setupAcceptanceTest,
  nspaceScenario,
  createModels,
  api,
  visit,
  page,
  currentURL,
  nspaceURL,
  nspaceEndpoint,
  assertVisibleCount,
  requestMade,
  clickParentBreadcrumb,
} from 'consul-ui/tests/helpers/acceptance';

const PROXY = {
  ServiceName: 'service-0-proxy',
  Node: 'node-0',
  ServiceID: 'service-a-proxy',
};

// A listing of instances from 2 services would never happen in Consul, but it
// satisfies the mocking needs for the instances and proxy requests at once.
const INSTANCES = [
  {
    Service: { Service: 'service-0', ID: 'service-a' },
    Node: { Node: 'node-0' },
    Checks: [{ Status: 'critical' }],
  },
  {
    Service: { Service: 'service-0', ID: 'service-b' },
    Node: { Node: 'node-0' },
    Checks: [{ Status: 'passing' }],
  },
  {
    Service: { Service: 'service-0-proxy', ID: 'service-a-proxy' },
    Node: { Node: 'node-0' },
    Checks: [{ Status: 'passing' }],
  },
];

module(
  'Acceptance | dc / services / instances / navigation: Instance Navigation',
  function (hooks) {
    setupAcceptanceTest(hooks);

    nspaceScenario(
      'Scenario: Clicking an instance in the listing and back again',
      async function (assert, nspace) {
        createModels('dc', 1, 'dc-1');
        api.server.createList('proxy', 1, PROXY);
        api.server.createList('instance', 3, INSTANCES);

        const detailURL = '/dc-1/services/service-0/instances/node-0/service-a/health-checks';
        const assertInstanceRequests = () => {
          requestMade(
            assert,
            'GET',
            nspaceEndpoint(
              '/v1/catalog/connect/service-0?dc=dc-1&merge-central-config&ns=@namespace',
              nspace
            )
          );
          requestMade(
            assert,
            'GET',
            nspaceEndpoint(
              '/v1/health/service/service-0-proxy?dc=dc-1&merge-central-config&ns=@namespace',
              nspace
            )
          );
        };

        await visit('service', { dc: 'dc-1', service: 'service-0' }, { nspace });
        await page().tabs.instances();
        assert.equal(currentURL(), nspaceURL(nspace, '/dc-1/services/service-0/instances'));
        assertVisibleCount(assert, page().instances, 3, 'instances');

        await page().instances.objectAt(0).instance();
        assertInstanceRequests();
        assert.equal(currentURL(), nspaceURL(nspace, detailURL));

        // The nearest ancestor breadcrumb returns to the service's instances tab.
        // (The old back link landed on the topology tab instead.)
        await clickParentBreadcrumb();
        assert.equal(currentURL(), nspaceURL(nspace, '/dc-1/services/service-0/instances'));

        await page().tabs.instances();
        await page().instances.objectAt(0).instance();
        assertInstanceRequests();
        assert.equal(currentURL(), nspaceURL(nspace, detailURL));
      }
    );
  }
);
