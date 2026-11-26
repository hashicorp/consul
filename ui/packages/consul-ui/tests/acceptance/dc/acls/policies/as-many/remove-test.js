/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

// Native QUnit port of tests/acceptance/dc/acls/policies/as-many/remove.feature.

import { module } from 'qunit';

import {
  setupAcceptanceTest,
  nspaceScenario,
  nspaceURL,
  nspaceEndpoint,
  createModels,
  visit,
  page,
  submit,
  currentURL,
  assertVisibleCount,
  requestMade,
} from 'consul-ui/tests/helpers/acceptance';

module('Acceptance | dc / acls / policies / as many / remove: Remove', function (hooks) {
  setupAcceptanceTest(hooks);

  ['token', 'role'].forEach((model) => {
    nspaceScenario(
      `Scenario: Removing policies as children of the ${model} page`,
      async function (assert, nspace) {
        createModels('dc', 1, 'datacenter');
        createModels(model, 1, {
          ServiceIdentities: null,
          NodeIdentities: null,
          Policies: [{ Name: 'Policy', ID: '00000000-0000-0000-0000-000000000001' }],
        });

        await visit(model, { dc: 'datacenter', [model]: 'key' }, { nspace });
        assert.equal(currentURL(), nspaceURL(nspace, `/datacenter/acls/${model}s/key`));
        assertVisibleCount(assert, page().policies.selectedOptions, 1, 'policy models');

        const policy = page().policies.selectedOptions.objectAt(0);
        await policy.expand();
        // Until we have a reliable way of mocking out and controlling
        // IntersectionObserver, the GET for the expanded policy isn't asserted.
        await policy.delete();
        await policy.confirmDelete();
        assertVisibleCount(assert, page().policies.selectedOptions, 0, 'policy models');

        await submit();

        requestMade(
          assert,
          'PUT',
          nspaceEndpoint(`/v1/acl/${model}/key?dc=datacenter&ns=@!namespace`, nspace),
          { body: { Policies: [] } }
        );
        assert.equal(currentURL(), nspaceURL(nspace, `/datacenter/acls/${model}s`));
      }
    );
  });
});
