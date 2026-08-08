/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

// Native QUnit port of tests/acceptance/dc/nodes/sessions/invalidate.feature.
// Invalidation is now confirmed through an HDS critical Modal
// (Consul::LockSession::Table) rather than the legacy inline dialog: clicking a
// row's "Invalidate" button opens the modal, and "Confirm Invalidation" issues
// the destroy request.

import { module } from 'qunit';
import { click } from '@ember/test-helpers';

import {
  setupAcceptanceTest,
  nspaceScenario,
  api,
  visit,
  lastNthRequest,
} from 'consul-ui/tests/helpers/acceptance';

const SESSION_ID = '7bbbd8bb-fff3-4292-b6e3-cfedd788546a';

const seedSessions = () => {
  api.server.createList('dc', 1, 'dc1');
  api.server.createList('node', 1, { ID: 'node-0' });
  api.server.createList('session', 2, [
    { ID: SESSION_ID },
    { ID: '7ccd0bd7-a5e0-41ae-a33e-ed3793d803b2' },
  ]);
};

// Navigate to the lock-sessions tab, then open and confirm the invalidate modal
// for the first session row.
const invalidateFirstSession = async (nspace) => {
  await visit('node', { dc: 'dc1', node: 'node-0' }, { nspace });
  await click('[data-test-tab="tab_lock-sessions"] a');
  await click('.consul-lock-session-table [data-test-delete]');
  await click('[data-test-confirm-delete]');
};

module('Acceptance | dc / nodes / sessions / invalidate', function (hooks) {
  setupAcceptanceTest(hooks);

  nspaceScenario('Scenario: Invalidating the lock session', async function (assert, nspace) {
    seedSessions();

    await invalidateFirstSession(nspace);

    const request = lastNthRequest(0, 'PUT');
    assert.ok(
      request && new RegExp(`/v1/session/destroy/${SESSION_ID}`).test(request.url),
      `a PUT request was made to destroy session ${SESSION_ID} (got: ${request && request.url})`
    );
    assert.dom('[data-notification]').hasClass('hds-toast');
    assert.dom('[data-notification]').hasClass('hds-alert--color-success');
  });

  nspaceScenario(
    'Scenario: Invalidating a lock session and receiving an error',
    async function (assert, nspace) {
      seedSessions();
      // Force the destroy endpoint to fail so the error notification is shown.
      api.server.respondWith(`/v1/session/destroy/${SESSION_ID}`, { status: 500 });

      await invalidateFirstSession(nspace);

      assert.dom('[data-notification]').hasClass('hds-toast');
      assert.dom('[data-notification]').hasClass('hds-alert--color-critical');
    }
  );
});
