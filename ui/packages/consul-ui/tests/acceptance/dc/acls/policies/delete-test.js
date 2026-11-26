/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

// Native QUnit port of tests/acceptance/dc/acls/policies/delete.feature.
//
// The `.feature` also had a commented-out "Deleting a policy from the policy
// detail page errors" scenario (FIXME); it never ran and is not ported.

import { module } from 'qunit';

import {
  setupAcceptanceTest,
  nspaceScenario,
  nspaceEndpoint,
  createModels,
  respondWith,
  visit,
  page,
  requestMade,
} from 'consul-ui/tests/helpers/acceptance';

const ID = '1981f51d-301a-497b-89a0-05112ef02b4b';

module('Acceptance | dc / acls / policies / delete: Policy Delete', function (hooks) {
  setupAcceptanceTest(hooks);

  hooks.beforeEach(function () {
    createModels('dc', 1, 'datacenter');
  });

  nspaceScenario(
    'Scenario: Deleting a policy model from the policies listing page',
    async function (assert, nspace) {
      createModels('policy', 1, { ID });

      await visit('policies', { dc: 'datacenter' }, { nspace });
      const policy = page().policies.objectAt(0);
      await policy.actions();
      await policy.delete();
      await policy.confirmDelete();

      requestMade(
        assert,
        'DELETE',
        nspaceEndpoint(`/v1/acl/policy/${ID}?dc=datacenter&ns=@!namespace`, nspace)
      );
      assert.dom('[data-notification]').hasClass('hds-toast');
      assert.dom('[data-notification]').hasClass('hds-alert--color-success');
    }
  );

  nspaceScenario(
    'Scenario: Deleting a policy model from the policies listing page errors',
    async function (assert, nspace) {
      respondWith(`/v1/acl/policy/${ID}`, { status: 500 });
      createModels('policy', 1, { ID });

      await visit('policies', { dc: 'datacenter' }, { nspace });
      const policy = page().policies.objectAt(0);
      await policy.actions();
      await policy.delete();
      await policy.confirmDelete();

      assert.dom('[data-notification]').hasClass('hds-toast');
      assert.dom('[data-notification]').hasClass('hds-alert--color-critical');
    }
  );

  nspaceScenario(
    'Scenario: Deleting a policy from the policy detail page',
    async function (assert, nspace) {
      await visit('policy', { dc: 'datacenter', policy: ID }, { nspace });
      await page().delete();
      await page().deleteModal.confirmDelete();

      requestMade(
        assert,
        'DELETE',
        nspaceEndpoint(`/v1/acl/policy/${ID}?dc=datacenter&ns=@!namespace`, nspace)
      );
      assert.dom('[data-notification]').hasClass('hds-toast');
      assert.dom('[data-notification]').hasClass('hds-alert--color-success');
    }
  );
});
