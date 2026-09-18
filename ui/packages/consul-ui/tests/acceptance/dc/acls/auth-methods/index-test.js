/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

// Native QUnit port of tests/acceptance/dc/acls/auth-methods/index.feature.
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

module('Acceptance | dc / acls / auth-methods / index: ACL Auth Methods List', function (hooks) {
  setupAcceptanceTest(hooks);

  nspaceScenario('Listing auth methods', async function (assert, nspace) {
    api.server.createList('dc', 1, 'dc-1');
    api.server.createList('authMethod', 3);

    await visit('authMethods', { dc: 'dc-1' }, { nspace });

    assert.equal(currentURL(), nspaceURL(nspace, '/dc-1/acls/auth-methods'));
    assert.equal(document.title, 'Auth Methods - Consul');
    assert.equal(page().authMethods.length, 3, 'shows 3 auth methods');
  });

  nspaceScenario('Searching the Auth Methods', async function (assert, nspace) {
    api.server.createList('dc', 1, 'dc-1');
    api.server.createList('authMethod', 3, [
      { Name: 'kube', DisplayName: 'minikube' },
      { Name: 'agent', DisplayName: '' },
      { Name: 'node', DisplayName: 'mininode' },
    ]);

    await visit('authMethods', { dc: 'dc-1' }, { nspace });

    assert.equal(currentURL(), nspaceURL(nspace, '/dc-1/acls/auth-methods'));
    assert.equal(page().authMethods.length, 3, 'shows 3 auth methods before filtering');

    // Search by display name / name — uses the unambiguous FilterBar selector.
    await fillIn('.hds-filter-bar__search', 'kube');
    assert.equal(page().authMethods.length, 1, 'shows 1 auth method for "kube"');
    assert.equal(
      page().authMethods.objectAt(0).name,
      'minikube',
      'shows the auth method named "minikube"'
    );

    await fillIn('.hds-filter-bar__search', 'agent');
    assert.equal(page().authMethods.length, 1, 'shows 1 auth method for "agent"');
    assert.equal(
      page().authMethods.objectAt(0).name,
      'agent',
      'shows the auth method named "agent"'
    );

    await fillIn('.hds-filter-bar__search', 'ode');
    assert.equal(page().authMethods.length, 1, 'shows 1 auth method for "ode"');
    assert.equal(
      page().authMethods.objectAt(0).name,
      'mininode',
      'shows the auth method named "mininode"'
    );
  });
});
