/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

// Native QUnit port of tests/acceptance/settings/show.feature.

import { module } from 'qunit';

import {
  setupAcceptanceTest,
  nspaceScenario,
  api,
  visit,
  page,
  currentURL,
} from 'consul-ui/tests/helpers/acceptance';

module('Acceptance | settings / show: Show Settings Page', function (hooks) {
  setupAcceptanceTest(hooks);

  nspaceScenario(
    'I see the Blocking queries',
    async function (assert) {
      api.server.createList('dc', 1, 'dc1');

      await visit('settings');
      assert.equal(currentURL(), '/settings');

      assert.ok(page().blockingQueries, 'blockingQueries is shown');
    },
    { notNamespaceable: true }
  );

  nspaceScenario(
    'Setting CONSUL_UI_DISABLE_REALTIME hides Blocking Queries',
    async function (assert) {
      api.server.createList('dc', 1, 'datacenter');
      window.localStorage.setItem('CONSUL_UI_DISABLE_REALTIME', JSON.stringify(1));
      assert.strictEqual(
        window.localStorage.getItem('CONSUL_UI_DISABLE_REALTIME'),
        '1',
        'the setting is persisted'
      );

      await visit('settings');
      assert.equal(currentURL(), '/settings');

      assert.notOk(page().blockingQueries, 'blockingQueries is hidden');
    },
    { notNamespaceable: true }
  );
});
