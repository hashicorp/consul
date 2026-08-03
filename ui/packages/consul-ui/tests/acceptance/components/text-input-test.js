/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

// Native QUnit port of tests/acceptance/components/text-input.feature.
//
// The KV create flow changed in the new UI: there is no longer a /kv/create
// sub-route. Instead the "Create" button opens a flyout on the list page
// (via a query-param). The form fields live inside `dialog .hds-flyout__footer`
// (scoped by the page object) but the inputs themselves are in the flyout body.

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

    // The new KV design has no /kv/create sub-route; the create flyout opens
    // on the list page via the "Create" button.
    await visit('kvs', { dc: 'dc-1' }, { nspace });
    assert.equal(currentURL(), nspaceURL(nspace, '/dc-1/kv'));

    // Open the create flyout.
    await click('[data-test-create]');

    // Turn the Code Editor off so we can fill the value as plain text.
    await click('dialog [name=json]');

    await fillIn('dialog [name="additional"]', 'hi');
    await fillIn('dialog [name="value"]', 'there');

    assert.ok(page().submitIsEnabled, 'the submit button is enabled');
  });
});
