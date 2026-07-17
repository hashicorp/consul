/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

// Native QUnit port of tests/acceptance/dc/error.feature.

import { module } from 'qunit';

import {
  setupAcceptanceTest,
  nspaceScenario,
  nspaceURL,
  api,
  visit,
  page,
  currentURL,
} from 'consul-ui/tests/helpers/acceptance';

const services = [
  { Name: 'Service-0', Kind: null },
  { Name: 'Service-1', Kind: null },
  { Name: 'Service-2', Kind: null },
];

const visible = (collection) => collection.filter((item) => item.isVisible).length;

module('Acceptance | dc / error', function (hooks) {
  setupAcceptanceTest(hooks);

  nspaceScenario('Recovering from a dc 500 error', async function (assert, nspace) {
    api.server.createList('dc', 2, ['dc-1', 'dc-500']);
    api.server.createList('service', 3, services);
    api.server.respondWith('/v1/internal/ui/services', { status: 500 });

    await visit('services', { dc: 'dc-500' }, { nspace });
    assert.equal(currentURL(), nspaceURL(nspace, '/dc-500/services'));
    assert.equal(document.title, 'Consul');
    assert.strictEqual(page().error.status, '500');

    api.server.respondWith('/v1/internal/ui/services', { status: 200 });

    await page().navigation.dc();
    await page().navigation.dcs.objectAt(0).name();

    assert.equal(visible(page().services), 3, 'sees 3 service models');
  });
});
