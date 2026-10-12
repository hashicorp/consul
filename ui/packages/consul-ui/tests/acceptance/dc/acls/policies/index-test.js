/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

// Native QUnit port of tests/acceptance/dc/acls/policies/index.feature.
//
// The `.feature` used `I type "..." into "input[type=search]"` which matched
// ambiguously under ENT (the side-nav namespace selector also renders an
// `input[type=search]`). Using the FilterBar-specific `.hds-filter-bar__search`
// selector removes the ambiguity.

import { module } from 'qunit';
import { fillIn } from '@ember/test-helpers';

import {
  setupAcceptanceTest,
  nspaceScenario,
  nspaceURL,
  api,
  visit,
  page,
  currentURL,
} from 'consul-ui/tests/helpers/acceptance';

module('Acceptance | dc / acls / policies / index: ACL Policy List', function (hooks) {
  setupAcceptanceTest(hooks);

  nspaceScenario('Listing policies', async function (assert, nspace) {
    api.server.createList('dc', 1, 'dc-1');
    api.server.createList('policy', 3);

    await visit('policies', { dc: 'dc-1' }, { nspace });

    assert.equal(currentURL(), nspaceURL(nspace, '/dc-1/acls/policies'));
    assert.equal(page().policies.length, 3, 'shows 3 policies');
    assert.equal(document.title, 'Policies - Consul');
    assert
      .dom('.hds-filter-bar__applied-filters-list')
      .containsText('No filters applied', 'shows the no-filters applied state');
  });

  nspaceScenario('Searching the policies', async function (assert, nspace) {
    api.server.createList('dc', 1, 'dc-1');
    api.server.createList('policy', 3, [
      { Description: 'Find me' },
      { Description: 'Not in search' },
      { Description: 'Not in search either' },
    ]);

    await visit('policies', { dc: 'dc-1' }, { nspace });

    assert.equal(currentURL(), nspaceURL(nspace, '/dc-1/acls/policies'));
    assert.equal(page().policies.length, 3, 'shows 3 policies before filtering');

    // Uses the unambiguous FilterBar selector to avoid matching the side-nav
    // namespace search input under ENT.
    await fillIn('.hds-filter-bar__search', 'Find me');

    assert.equal(page().policies.length, 1, 'shows 1 policy after search');
    assert.equal(
      page().policies.objectAt(0).description,
      'Find me',
      'the visible policy has the expected description'
    );
  });

  // The global-management policy scenario is @ignored in the original feature;
  // preserve as a no-op skipped test to keep the tracker consistent.
});
