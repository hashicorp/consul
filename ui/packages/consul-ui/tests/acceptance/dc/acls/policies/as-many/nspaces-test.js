/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

// Native QUnit port of tests/acceptance/dc/acls/policies/as-many/nspaces.feature.
// The feature was `@onlyNamespaceable` and its single scenario was untitled
// (reported as "Scenario: "), so the native test is given a title.

import { module } from 'qunit';

import {
  setupAcceptanceTest,
  nspaceScenario,
  nspaceURL,
  createModels,
  visit,
  page,
  currentURL,
} from 'consul-ui/tests/helpers/acceptance';

module(
  'Acceptance | dc / acls / policies / as many / nspaces: As many for nspaces',
  function (hooks) {
    setupAcceptanceTest(hooks);

    nspaceScenario(
      'Scenario: A namespace policy cannot be a service identity',
      async function (assert, nspace) {
        createModels('dc', 1, 'dc1');
        createModels('nspace', 1, {
          Name: 'key',
          ACLs: { PolicyDefaults: null, RoleDefaults: null },
        });

        await visit('nspace', { dc: 'dc1', namespace: 'key' }, { nspace });
        assert.equal(currentURL(), nspaceURL(nspace, '/dc1/namespaces/key'));

        await page().policies.create();
        assert
          .dom('#policies [data-test-radiobutton=template_service-identity]')
          .doesNotExist('there is no service identity option');
      },
      { onlyNamespaceable: true }
    );
  }
);
