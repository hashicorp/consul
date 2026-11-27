/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

// Skipped with `{ ignore: true }`: it was already ignored before the move to
// native QUnit.

import { module } from 'qunit';

import {
  setupAcceptanceTest,
  nspaceScenario,
  api,
  visit,
  submit,
  currentURL,
} from 'consul-ui/tests/helpers/acceptance';

module('Acceptance | settings / update: Update Settings', function (hooks) {
  setupAcceptanceTest(hooks);

  nspaceScenario(
    'I click Save without actually typing anything',
    async function (assert) {
      api.server.createList('dc', 1, 'datacenter');

      await visit('settings');
      assert.equal(currentURL(), '/settings');
      assert.strictEqual(window.localStorage.getItem('consul:token'), null);

      await submit();

      assert.strictEqual(window.localStorage.getItem('consul:token'), '');
      assert.equal(currentURL(), '/settings');
      assert.dom('[data-notification]').hasClass('hds-toast');
      assert.dom('[data-notification]').hasClass('hds-alert--color-success');
    },
    { ignore: true }
  );
});
