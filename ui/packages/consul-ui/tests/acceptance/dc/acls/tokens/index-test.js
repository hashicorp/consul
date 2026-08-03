/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

// Native QUnit port of tests/acceptance/dc/acls/tokens/index.feature.
//
// The `.feature` used `I type "..." into "input[type=search]"` which matched
// ambiguously under ENT (the side-nav namespace selector also renders an
// `input[type=search]`). Using the FilterBar-specific `.hds-filter-bar__search`
// selector removes the ambiguity.

import { module, skip } from 'qunit';
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

module('Acceptance | dc / acls / tokens / index: ACL Token List', function (hooks) {
  setupAcceptanceTest(hooks);

  nspaceScenario('I see the tokens', async function (assert, nspace) {
    api.server.createList('dc', 1, 'dc-1');
    api.server.createList('token', 3);

    await visit('tokens', { dc: 'dc-1' }, { nspace });

    assert.equal(currentURL(), nspaceURL(nspace, '/dc-1/acls/tokens'));
    assert.equal(document.title, 'Tokens - Consul');
    assert.equal(page().tokens.length, 3, 'shows 3 tokens');
  });

  nspaceScenario('Viewing tokens with no write access', async function (assert, nspace) {
    api.server.createList('dc', 1, 'dc-1');
    api.server.createList('token', 3);
    // Mirror the yadda `permissions from yaml` step via the api-double cookie
    // mechanism (direct document.cookie writes are not picked up by the env service).
    api.server.setCookie('CONSUL_RESOURCE_ACL_WRITE', false);

    await visit('tokens', { dc: 'dc-1' }, { nspace });

    assert.equal(currentURL(), nspaceURL(nspace, '/dc-1/acls/tokens'));
    assert
      .dom('[data-test-create]')
      .doesNotExist('create button is not shown without write access');
  });

  nspaceScenario('Searching the tokens', async function (assert, nspace) {
    api.server.createList('dc', 1, 'dc-1');
    api.server.createList('token', 4, [
      {
        Description: 'Description Search',
        Legacy: false,
        ServiceIdentities: [{ ServiceName: 'not-in-sisearch' }],
        Policies: [{ Name: 'not-in-Polsearch' }],
        Roles: [{ Name: 'not-in-rolesearch' }],
      },
      {
        Description: 'Not in descsearch',
        Legacy: false,
        ServiceIdentities: [{ ServiceName: 'not-in-sisearch' }],
        Policies: [{ Name: 'Policy-Search' }],
        Roles: [{ Name: 'not-in-rolesearch-either' }],
      },
      {
        Description: 'Not in descsearch either',
        Legacy: false,
        ServiceIdentities: [{ ServiceName: 'not-in-sisearch' }],
        Policies: [{ Name: 'not-int-Polsearch-either' }],
        Roles: [{ Name: 'Role-Search' }],
      },
      {
        Description: 'Not in descsearch either',
        Legacy: false,
        ServiceIdentities: [{ ServiceName: 'Si-Search' }],
        Policies: [{ Name: 'not-int-Polsearch-either' }],
        Roles: [{ Name: 'not-in-rolesearch-either' }],
      },
    ]);

    await visit('tokens', { dc: 'dc-1' }, { nspace });

    assert.equal(currentURL(), nspaceURL(nspace, '/dc-1/acls/tokens'));
    assert.equal(page().tokens.length, 4, 'shows 4 tokens before filtering');

    // Uses the unambiguous FilterBar selector to avoid matching the side-nav
    // namespace search input under ENT.
    await fillIn('.hds-filter-bar__search', 'Description');
    assert.equal(page().tokens.length, 1, 'shows 1 token for "Description"');
    assert.equal(
      page().tokens.objectAt(0).description,
      'Description Search',
      'the visible token has description "Description Search"'
    );

    // Policy/role/service-identity assertions use aria-label since the HDS Badge
    // wraps the text inside nested spans that make .textContent unreliable.
    await fillIn('.hds-filter-bar__search', 'Policy-Search');
    assert.equal(page().tokens.length, 1, 'shows 1 token for "Policy-Search"');
    assert
      .dom('.consul-token-list [data-test-tabular-row] [data-test-policy][data-type="policy"]')
      .hasAttribute('aria-label', 'Policy-Search', 'the visible token has policy "Policy-Search"');

    await fillIn('.hds-filter-bar__search', 'Role-Search');
    assert.equal(page().tokens.length, 1, 'shows 1 token for "Role-Search"');
    assert
      .dom('.consul-token-list [data-test-tabular-row] [data-test-policy][data-type="role"]')
      .hasAttribute('aria-label', 'Role-Search', 'the visible token has role "Role-Search"');

    await fillIn('.hds-filter-bar__search', 'Si-Search');
    assert.equal(page().tokens.length, 1, 'shows 1 token for "Si-Search"');
    assert
      .dom(
        '.consul-token-list [data-test-tabular-row] [data-test-policy][data-type="policy-service-identity"]'
      )
      .hasAttribute(
        'aria-label',
        'Service Identity: Si-Search',
        'the visible token has service identity "Si-Search"'
      );
  });

  nspaceScenario(
    'I see the legacy message if I have one legacy token',
    async function (assert, nspace) {
      api.server.createList('dc', 1, 'dc-1');
      api.server.createList('token', 3, [{ Legacy: true }, { Legacy: false }, { Legacy: false }]);

      await visit('tokens', { dc: 'dc-1' }, { nspace });

      assert.equal(currentURL(), nspaceURL(nspace, '/dc-1/acls/tokens'));
      assert.dom('[data-test-notification-update]').exists('shows the legacy update notice');
      assert.equal(page().tokens.length, 3, 'shows 3 tokens');
    }
  );

  nspaceScenario(
    "I don't see the legacy message if I have no legacy tokens",
    async function (assert, nspace) {
      api.server.createList('dc', 1, 'dc-1');
      api.server.createList('token', 3, [{ Legacy: false }, { Legacy: false }, { Legacy: false }]);

      await visit('tokens', { dc: 'dc-1' }, { nspace });

      assert.equal(currentURL(), nspaceURL(nspace, '/dc-1/acls/tokens'));
      assert
        .dom('[data-test-notification-update]')
        .doesNotExist('does not show the legacy update notice');
      assert.equal(page().tokens.length, 3, 'shows 3 tokens');
    }
  );

  // Placeholder — sorting scenarios live in dc/acls/tokens/sorting.feature (not migrated yet).
  skip('sorting scenarios are covered in dc/acls/tokens/sorting.feature', function () {});
});
