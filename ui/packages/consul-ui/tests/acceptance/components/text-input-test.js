/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

// Native QUnit port of tests/acceptance/components/text-input.feature.

import { module } from 'qunit';
import { click, fillIn } from '@ember/test-helpers';

import {
  setupAcceptanceTest,
  nspaceScenario,
  nspaceURL,
  api,
  visit,
  page,
  currentURL,
} from 'consul-ui/tests/helpers/acceptance';

module('Acceptance | components / text-input: Text input', function (hooks) {
  setupAcceptanceTest(hooks);

  nspaceScenario('KV page', async function (assert, nspace) {
    api.server.createList('dc', 1, 'dc-1');

    await visit('kv', { dc: 'dc-1' }, { nspace });
    assert.equal(currentURL(), nspaceURL(nspace, '/dc-1/kv/create'));

    // Turn the Code Editor off so we can fill the value easier.
    await click('[name=json]');

    await fillIn('[name="additional"]', 'hi');
    await fillIn('[name="value"]', 'there');

    assert.ok(page().submitIsEnabled, 'the submit button is enabled');
  });
});
