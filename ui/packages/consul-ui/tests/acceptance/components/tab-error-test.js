/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

// An error loading a tab's content renders inside the tab, under the page's
// own breadcrumbs and heading, rather than as a second nested page.

import { module } from 'qunit';
import { click } from '@ember/test-helpers';

import {
  setupAcceptanceTest,
  nspaceScenario,
  api,
  visit,
  currentURL,
} from 'consul-ui/tests/helpers/acceptance';

module('Acceptance | components / tab error', function (hooks) {
  setupAcceptanceTest(hooks);

  nspaceScenario(
    'An error loading a tab renders the error state inside the page',
    async function (assert, nspace) {
      api.server.createList('dc', 1, 'dc-1');
      api.server.createList('node', 1, { ID: 'node-0' });
      api.server.respondWith('/v1/session/node/node-0', { status: 500 });

      await visit('node', { dc: 'dc-1', node: 'node-0' }, { nspace });
      await click('[data-test-tab="tab_lock-sessions"] button');

      assert.ok(currentURL().endsWith('/nodes/node-0/lock-sessions'), 'the tab changed route');
      assert.dom('[data-test-status]').hasText('Error 500', 'the error state is shown');
      assert.dom('.app-view').exists({ count: 1 }, 'no nested AppView is rendered');
      assert.dom('h1').exists({ count: 1 }, 'the page has a single h1');
      assert.dom('h1').includesText('node-0', 'the h1 is still the page heading');
      assert.dom('[data-test-breadcrumbs]').exists({ count: 1 }, 'breadcrumbs are not duplicated');
    }
  );
});
