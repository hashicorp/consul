/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

// Native QUnit port of tests/acceptance/dc/nodes/sessions/list.feature.
// The lock-sessions tab now renders an HDS table (Consul::LockSession::Table)
// rather than the legacy card list, so the assertions target the table cells.

import { module } from 'qunit';
import { click } from '@ember/test-helpers';

import {
  setupAcceptanceTest,
  nspaceScenario,
  api,
  visit,
  currentURL,
  nspaceURL,
} from 'consul-ui/tests/helpers/acceptance';

// The node page redirects to the health-checks tab; navigate on to the
// lock-sessions tab (whose DataLoader only fetches once that tab is active).
const visitSessions = async (nspace) => {
  await visit('node', { dc: 'dc1', node: 'node-0' }, { nspace });
  await click('[data-test-tab="tab_lock-sessions"] a');
};

const cellText = (selector) =>
  [...document.querySelectorAll(`.consul-lock-session-table ${selector}`)].map((el) =>
    el.textContent.trim()
  );

module('Acceptance | dc / nodes / sessions / list', function (hooks) {
  setupAcceptanceTest(hooks);

  nspaceScenario('Scenario: Given 2 sessions with string TTLs', async function (assert, nspace) {
    api.server.createList('dc', 1, 'dc1');
    api.server.createList('node', 1, { ID: 'node-0' });
    api.server.createList('session', 2, [{ TTL: '30s' }, { TTL: '60m' }]);

    await visitSessions(nspace);

    assert.equal(currentURL(), nspaceURL(nspace, '/dc1/nodes/node-0/lock-sessions'));
    assert.dom('[data-test-tab="tab_lock-sessions"]').hasClass('selected');
    assert.deepEqual(
      cellText('[data-test-session-ttl]'),
      ['30s', '60m'],
      'the TTL column renders each session TTL in order'
    );
  });

  nspaceScenario(
    'Scenario: Given 3 sessions with LockDelay in nanoseconds',
    async function (assert, nspace) {
      api.server.createList('dc', 1, 'dc1');
      api.server.createList('node', 1, { ID: 'node-0' });
      api.server.createList('session', 3, [
        { LockDelay: 120000 },
        { LockDelay: 18000000 },
        { LockDelay: 15000000000 },
      ]);

      await visitSessions(nspace);

      assert.deepEqual(
        cellText('[data-test-session-delay]'),
        ['120µs', '18ms', '15s'],
        'the Lock delay column renders each LockDelay as a human duration'
      );
    }
  );

  nspaceScenario(
    'Scenario: Given 0 sessions with ACLs enabled',
    async function (assert, nspace) {
      api.server.createList('dc', 1, 'dc1');
      api.server.createList('node', 1, { ID: 'node-0' });
      api.server.createList('session', 0);

      await visitSessions(nspace);

      assert
        .dom('.empty-state p')
        .includesText(
          'you may not have key:read or session:read permissions',
          'the empty state mentions the ACL permissions when ACLs are enabled'
        );
      assert.dom('[data-test-empty-state-login]').exists('the login CTA is shown when ACLs are enabled');
    }
  );

  nspaceScenario(
    'Scenario: Given 0 sessions with ACLs disabled',
    async function (assert, nspace) {
      // The env service reads CONSUL_ACLS_ENABLED from document.cookie (not the
      // api-double server cookie jar), so disable ACLs via document.cookie —
      // exactly like the original yadda "ACLs are disabled" step. setupAcceptanceTest's
      // reset() wipes all cookies between tests so this does not leak.
      document.cookie = 'CONSUL_ACLS_ENABLE=0';
      api.server.createList('dc', 1, 'dc1');
      api.server.createList('node', 1, { ID: 'node-0' });
      api.server.createList('session', 0);

      await visitSessions(nspace);

      assert
        .dom('.empty-state p')
        .doesNotIncludeText(
          'you may not have key:read or session:read permissions',
          'the empty state omits the ACL permissions line when ACLs are disabled'
        );
      assert
        .dom('[data-test-empty-state-login]')
        .doesNotExist('the login CTA is hidden when ACLs are disabled');
    },
    { notNamespaceable: true }
  );
});
