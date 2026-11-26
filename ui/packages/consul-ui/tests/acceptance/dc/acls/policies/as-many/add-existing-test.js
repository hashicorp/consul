/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

// Native QUnit port of tests/acceptance/dc/acls/policies/as-many/add-existing.feature.

import { module } from 'qunit';
import { click, fillIn } from '@ember/test-helpers';

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

module(
  'Acceptance | dc / acls / policies / as many / add existing: Add existing policy',
  function (hooks) {
    setupAcceptanceTest(hooks);

    ['role'].forEach((model) => {
      nspaceScenario(
        `Scenario: Adding an existing policy as a child of ${model}`,
        async function (assert, nspace) {
          createModels('dc', 1, 'datacenter');
          createModels(model, 1, { Policies: null, ServiceIdentities: null, NodeIdentities: null });
          createModels('policy', 2, [
            { ID: 'policy-2', Name: 'Policy 2' },
            { ID: 'policy-1', Name: 'Policy 1' },
          ]);

          await visit(model, { dc: 'datacenter', [model]: 'key' }, { nspace });
          assert.equal(currentURL(), nspaceURL(nspace, `/datacenter/acls/${model}s/key`));

          await click('form > #policies .ember-power-select-trigger');
          assert.dom('.ember-power-select-option:first-child').includesText('Policy 1');
          await fillIn('.ember-power-select-search-input', 'Policy 1');
          await click('.ember-power-select-option:first-child');
          assertVisibleCount(assert, page().policies.selectedOptions, 1, 'policy models');

          await click('form > #policies .ember-power-select-trigger');
          await fillIn('.ember-power-select-search-input', 'Policy 2');
          await click('.ember-power-select-option:first-child');
          assertVisibleCount(assert, page().policies.selectedOptions, 2, 'policy models');

          await submit();

          requestMade(
            assert,
            'PUT',
            nspaceEndpoint(`/v1/acl/${model}/key?dc=datacenter&ns=@!namespace`, nspace),
            {
              body: {
                Policies: [
                  { ID: 'policy-1', Name: 'Policy 1' },
                  { ID: 'policy-2', Name: 'Policy 2' },
                ],
              },
            }
          );
          assert.equal(currentURL(), nspaceURL(nspace, `/datacenter/acls/${model}s`));
          assert.dom('[data-notification]').hasClass('hds-toast');
          assert.dom('[data-notification]').hasClass('hds-alert--color-success');
        }
      );
    });
  }
);
