/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

// Native QUnit port of tests/acceptance/components/text-input.feature.
//
// The KV create flow uses a dedicated /kv/create sub-route (dc.kv.root-create).
// Clicking [data-test-create] on the list page navigates there; the form
// fields are in the page body with no dialog wrapper.

import { module } from 'qunit';
import { click, fillIn, currentURL } from '@ember/test-helpers';
import { assert as qunitAssert } from '@ember/test-helpers';

import {
  setupAcceptanceTest,
  nspaceScenario,
  nspaceURL,
  api,
  visit,
} from 'consul-ui/tests/helpers/acceptance';

module('Acceptance | components / text-input: Text input', function (hooks) {
  setupAcceptanceTest(hooks);

  nspaceScenario('KV page', async function (assert, nspace) {
    api.server.createList('dc', 1, 'dc-1');

    await visit('kvs', { dc: 'dc-1' }, { nspace });
    assert.equal(currentURL(), nspaceURL(nspace, '/dc-1/kv'));

    // Navigate to the create route via the Create button.
    await click('[data-test-create]');

    // Turn the Code Editor off so we can fill the value as plain text.
    await click('[name=json]');

    await fillIn('[name="additional"]', 'hi');
    await fillIn('[name="value"]', 'there');

    assert.dom('main [type=submit]').isNotDisabled('the submit button is enabled');
  });
});
