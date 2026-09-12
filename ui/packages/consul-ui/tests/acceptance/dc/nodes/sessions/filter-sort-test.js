/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

// Coverage for the lock-sessions HDS table capabilities introduced by the
// migration (these had no equivalent in the legacy card list): client-side
// column sorting via the sortable headers, the Behaviour filter, and the
// free-text search — the latter two driven through the tab's query params, the
// same way the toolbar's FilterBar pushes them into the data layer.

import { module } from 'qunit';
import { click, visit as visitURL } from '@ember/test-helpers';

import {
  setupAcceptanceTest,
  nspaceScenario,
  api,
  visit,
  nspaceURL,
} from 'consul-ui/tests/helpers/acceptance';

// Sortable column header positions (1-based) in Consul::LockSession::Table.
const HEADER = { name: 1, lockDelay: 3, ttl: 4, behavior: 5 };
const sortHeader = (column) =>
  `.consul-lock-session-table thead th:nth-child(${HEADER[column]}) button`;

// The names rendered in the table body, top to bottom.
const renderedNames = () =>
  [...document.querySelectorAll('.consul-lock-session-table [data-test-session-name]')].map((el) =>
    el.textContent.trim()
  );

const seed = (sessions) => {
  api.server.createList('dc', 1, 'dc1');
  api.server.createList('node', 1, { ID: 'node-0' });
  api.server.createList('session', sessions.length, sessions);
};

// Navigate to the lock-sessions tab from the node page (the tab's DataLoader
// only fetches once it is active).
const visitSessions = async (nspace) => {
  await visit('node', { dc: 'dc1', node: 'node-0' }, { nspace });
  await click('[data-test-tab="tab_lock-sessions"] a');
};

module('Acceptance | dc / nodes / sessions / filter + sort', function (hooks) {
  setupAcceptanceTest(hooks);

  nspaceScenario(
    'Scenario: Sorting by Name toggles ascending / descending',
    async function (assert, nspace) {
      seed([{ Name: 'zeta' }, { Name: 'alpha' }, { Name: 'mike' }]);

      await visitSessions(nspace);

      // First click sorts ascending.
      await click(sortHeader('name'));
      assert.deepEqual(renderedNames(), ['alpha', 'mike', 'zeta'], 'Name ascending');

      // Clicking the same header again flips to descending.
      await click(sortHeader('name'));
      assert.deepEqual(renderedNames(), ['zeta', 'mike', 'alpha'], 'Name descending');
    }
  );

  nspaceScenario(
    'Scenario: Sorting by Behaviour orders by the behaviour value',
    async function (assert, nspace) {
      seed([
        { Name: 'releases', Behavior: 'release' },
        { Name: 'deletes', Behavior: 'delete' },
      ]);

      await visitSessions(nspace);

      // Ascending: 'delete' sorts before 'release'.
      await click(sortHeader('behavior'));
      assert.deepEqual(renderedNames(), ['deletes', 'releases'], 'Behaviour ascending');

      // Descending: 'release' before 'delete'.
      await click(sortHeader('behavior'));
      assert.deepEqual(renderedNames(), ['releases', 'deletes'], 'Behaviour descending');
    }
  );

  nspaceScenario(
    'Scenario: The Behaviour filter narrows the table to matching sessions',
    async function (assert, nspace) {
      seed([
        { Name: 'r1', Behavior: 'release' },
        { Name: 'd1', Behavior: 'delete' },
        { Name: 'r2', Behavior: 'release' },
      ]);

      await visitURL(nspaceURL(nspace, '/dc1/nodes/node-0/lock-sessions?behavior=release'));

      assert.deepEqual(
        renderedNames().sort(),
        ['r1', 'r2'],
        'only the "release" sessions are shown'
      );
      assert
        .dom('.consul-lock-session-toolbar .hds-filter-bar__applied-filters')
        .includesText('Release', 'the applied-filter tag reflects the Behaviour filter');
    }
  );

  nspaceScenario(
    'Scenario: Free-text search narrows the table by name',
    async function (assert, nspace) {
      seed([{ Name: 'alpha' }, { Name: 'beta' }, { Name: 'alphabet' }]);

      await visitURL(nspaceURL(nspace, '/dc1/nodes/node-0/lock-sessions?filter=alpha'));

      assert.deepEqual(
        renderedNames().sort(),
        ['alpha', 'alphabet'],
        'only sessions whose name matches the search term are shown'
      );
    }
  );
});
