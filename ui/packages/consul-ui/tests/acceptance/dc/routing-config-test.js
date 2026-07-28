/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

// Native QUnit port of tests/acceptance/dc/routing-config.feature.

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

module('Acceptance | dc / routing-config', function (hooks) {
  setupAcceptanceTest(hooks);

  nspaceScenario('Viewing a routing config', async function (assert, nspace) {
    api.server.createList('dc', 1, 'dc1');

    await visit('routingConfig', { dc: 'dc1', name: 'virtual-1' }, { nspace });
    assert.equal(currentURL(), nspaceURL(nspace, '/dc1/routing-config/virtual-1'));
    // A missing error element means no error was rendered (the page loaded), so
    // treat "not found" the same as the yadda `I don't see status ... like 404`.
    let status;
    try {
      status = page().error.status;
    } catch (e) {
      status = null;
    }
    assert.notEqual(status, '404', "doesn't see a 404 error");
    assert.equal(document.title, 'virtual-1 - Consul');
  });

  nspaceScenario('Viewing a source pill', async function (assert, nspace) {
    api.server.createList('dc', 1, 'dc1');

    await visit('routingConfig', { dc: 'dc1', name: 'virtual-1' }, { nspace });
    assert.ok(page().source, 'sees the source pill');
  });
});
