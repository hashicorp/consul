/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

// Activating a route-backed TabNav tab keeps keyboard focus on that tab (the
// WAI-ARIA tabs pattern), while every other route change still moves focus to
// the top of the page via Hds::AppHeader's NavigationNarrator.

import { module } from 'qunit';
import { click } from '@ember/test-helpers';

import {
  setupAcceptanceTest,
  nspaceScenario,
  api,
  visit,
  currentURL,
} from 'consul-ui/tests/helpers/acceptance';

const NARRATOR = '#ember-a11y-refocus-nav-message';
const tabButton = (name) => `[data-test-tab="tab_${name}"] button`;

module('Acceptance | components / tab-nav focus', function (hooks) {
  setupAcceptanceTest(hooks);

  nspaceScenario(
    'Activating a tab keeps focus on the tab, other navigation moves it',
    async function (assert, nspace) {
      api.server.createList('dc', 1, 'dc-1');
      api.server.createList('node', 1, { ID: 'node-0' });

      await visit('node', { dc: 'dc-1', node: 'node-0' }, { nspace });
      assert.dom(tabButton('health-checks')).hasAttribute('aria-selected', 'true');

      await click(tabButton('lock-sessions'));

      assert.ok(currentURL().endsWith('/nodes/node-0/lock-sessions'), 'the tab changed route');
      assert.dom(tabButton('lock-sessions')).isFocused('focus stays on the activated tab');
      assert
        .dom(`#${document.querySelector(tabButton('lock-sessions')).getAttribute('aria-controls')}`)
        .hasAttribute('role', 'tabpanel')
        .doesNotHaveAttribute('hidden', 'the activated tab panel is shown');

      await click(tabButton('metadata'));

      assert.dom(tabButton('metadata')).isFocused('focus stays on the next activated tab too');

      await click('[data-test-breadcrumb-item]:not([data-test-breadcrumb-current]) a');

      assert.dom(NARRATOR).isFocused('non-tab navigation moves focus to the top of the page');
    }
  );
});
