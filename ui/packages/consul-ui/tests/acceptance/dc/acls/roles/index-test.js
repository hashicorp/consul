/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

// Native QUnit port of tests/acceptance/dc/acls/roles/index.feature.
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

module('Acceptance | dc / acls / roles / index: ACL Roles List', function (hooks) {
  setupAcceptanceTest(hooks);

  nspaceScenario('Listing roles', async function (assert, nspace) {
    api.server.createList('dc', 1, 'dc-1');
    api.server.createList('role', 3);

    await visit('roles', { dc: 'dc-1' }, { nspace });

    assert.equal(currentURL(), nspaceURL(nspace, '/dc-1/acls/roles'));
    assert.equal(page().roles.length, 3, 'shows 3 roles');
    assert.equal(document.title, 'Roles - Consul');
  });

  nspaceScenario('Searching the roles', async function (assert, nspace) {
    api.server.createList('dc', 1, 'dc-1');
    api.server.createList('role', 3, [
      {
        Description: 'Description Search',
        Policies: [{ Name: 'not-in-Polsearch' }],
        ServiceIdentities: [{ ServiceName: 'not-in-sisearch' }],
      },
      {
        Description: 'Not in descsearch',
        Policies: [{ Name: 'Policy-Search' }],
        ServiceIdentities: [{ ServiceName: 'not-in-sisearch' }],
      },
      {
        Description: 'Not in descsearch either',
        Policies: [{ Name: 'not-in-Polsearch-either' }],
        ServiceIdentities: [{ ServiceName: 'Si-Search' }],
      },
    ]);

    await visit('roles', { dc: 'dc-1' }, { nspace });

    assert.equal(currentURL(), nspaceURL(nspace, '/dc-1/acls/roles'));
    assert.equal(page().roles.length, 3, 'shows 3 roles before filtering');

    // Description search — uses the unambiguous FilterBar selector.
    await fillIn('.hds-filter-bar__search', 'Description');
    assert.equal(page().roles.length, 1, 'shows 1 role for "Description"');
    assert.equal(
      page().roles.objectAt(0).description,
      'Description Search',
      'the visible role has description "Description Search"'
    );

    // Policy name search — assert via aria-label since the HDS Badge wraps the
    // text inside nested spans that make .textContent unreliable.
    await fillIn('.hds-filter-bar__search', 'Policy-Search');
    assert.equal(page().roles.length, 1, 'shows 1 role for "Policy-Search"');
    assert
      .dom('.consul-role-list [data-test-tabular-row] [data-test-policy][data-type="policy"]')
      .hasAttribute('aria-label', 'Policy-Search', 'the visible role has policy "Policy-Search"');

    // Service identity search.
    await fillIn('.hds-filter-bar__search', 'Si-Search');
    assert.equal(page().roles.length, 1, 'shows 1 role for "Si-Search"');
    assert
      .dom(
        '.consul-role-list [data-test-tabular-row] [data-test-policy][data-type="policy-service-identity"]'
      )
      .hasAttribute(
        'aria-label',
        'Service Identity: Si-Search',
        'the visible role has service identity "Si-Search"'
      );
  });
});
