/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module } from 'qunit';

import {
  setupAcceptanceTest,
  nspaceScenario,
  nspaceURL,
  nspaceEndpoint,
  createModels,
  visit,
  page,
  currentURL,
  assertVisibleCount,
  requestMade,
} from 'consul-ui/tests/helpers/acceptance';

module('Acceptance | dc / services / list', function (hooks) {
  setupAcceptanceTest(hooks);

  nspaceScenario('Scenario: Listing service', async function (assert, nspace) {
    createModels('dc', 1, 'dc-1');
    createModels(
      'service',
      3,
      [0, 1, 2].map((i) => ({ Name: `Service-${i}`, Kind: null }))
    );

    await visit('services', { dc: 'dc-1' }, { nspace });
    assert.equal(currentURL(), nspaceURL(nspace, '/dc-1/services'));
    requestMade(
      assert,
      'GET',
      nspaceEndpoint('/v1/internal/ui/services?dc=dc-1&ns=@namespace', nspace)
    );
    assertVisibleCount(assert, page().services, 3, 'service models');
  });

  nspaceScenario('Scenario: Listing peered service', async function (assert, nspace) {
    createModels('dc', 1, 'dc-1');
    createModels('service', 1, [{ Name: 'Service-0', Kind: null, PeerName: 'billing-app' }]);

    await visit('services', { dc: 'dc-1' }, { nspace });
    assert.equal(currentURL(), nspaceURL(nspace, '/dc-1/services'));
    assertVisibleCount(assert, page().services, 1, 'service models');
    const named = page().services.filter((item) => item.isVisible && item.name === 'Service-0');
    assert.equal(named.length, 1, 'sees Service-0');
  });
});
