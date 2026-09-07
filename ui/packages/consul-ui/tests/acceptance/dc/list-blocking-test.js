/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

// Native QUnit port of tests/acceptance/dc/list-blocking.feature.
//
// Listing pages live-update via blocking queries when Consul changes
// externally. The second scenario (detail pages with a listing) carried a
// `@ignore` (FIXME) annotation and is preserved as a skipped test.

import { module } from 'qunit';
import { click, waitUntil } from '@ember/test-helpers';

import {
  setupAcceptanceTest,
  nspaceScenario,
  nspaceURL,
  api,
  visit,
  page,
  currentURL,
} from 'consul-ui/tests/helpers/acceptance';

const visible = (collection) => collection.filter((item) => item.isVisible).length;

// Wait for the (blocking-query driven) list to settle on `count` visible rows.
const waitForCount = (field, count) =>
  waitUntil(() => visible(page()[field]) === count, { timeout: 5000 });

module('Acceptance | dc / list-blocking', function (hooks) {
  setupAcceptanceTest(hooks);

  nspaceScenario('Viewing the listing pages for nodes', async function (assert, nspace) {
    api.server.createList('dc', 1, 'dc-1');
    api.server.createList('node', 3);
    api.server.setCookie('CONSUL_LATENCY', 100);

    await visit('nodes', { dc: 'dc-1' }, { nspace });
    assert.equal(currentURL(), nspaceURL(nspace, '/dc-1/nodes'));

    await waitForCount('nodes', 3);
    assert.equal(visible(page().nodes), 3, 'starts with 3 node models');

    api.server.createList('node', 5);
    await waitForCount('nodes', 5);
    assert.equal(visible(page().nodes), 5, 'live-updates to 5 node models');

    api.server.createList('node', 1);
    await waitForCount('nodes', 1);
    assert.equal(visible(page().nodes), 1, 'live-updates to 1 node model');

    api.server.createList('node', 0);
    await waitForCount('nodes', 0);
    assert.equal(visible(page().nodes), 0, 'live-updates to 0 node models');
  });

  // FIXME: preserved from the original @ignore annotation.
  nspaceScenario(
    'Viewing detail pages with a listing for service instances',
    async function (assert, nspace) {
      api.server.createList('dc', 1, 'dc-1');
      api.server.createList('instance', 3);
      api.server.setCookie('CONSUL_LATENCY', 100);

      await visit('service', { dc: 'dc-1', service: 'service' }, { nspace });
      await click('[data-test-tab="instances"] a');
      assert.equal(currentURL(), nspaceURL(nspace, '/dc-1/services/service/instances'));

      await waitForCount('instances', 3);
      api.server.createList('instance', 5);
      await waitForCount('instances', 5);
      api.server.createList('instance', 1);
      await waitForCount('instances', 1);
      api.server.createList('instance', 0);
      await waitUntil(
        () => document.querySelector('[data-notification]')?.textContent.includes('deregistered'),
        { timeout: 5000 }
      );
    },
    { ignore: true }
  );
});
