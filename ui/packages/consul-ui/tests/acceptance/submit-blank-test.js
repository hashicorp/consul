/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

// Native QUnit port of tests/acceptance/submit-blank.feature
// (the feature's `@ignore` "The button is disabled" placeholder was deleted)

import { module } from 'qunit';

import {
  setupAcceptanceTest,
  nspaceScenario,
  createModels,
  visit,
  page,
  currentURL,
  nspaceURL,
} from 'consul-ui/tests/helpers/acceptance';

module('Acceptance | submit-blank', function (hooks) {
  setupAcceptanceTest(hooks);

  [{ model: 'kv', slug: 'kv' }].forEach(({ model, slug }) => {
    nspaceScenario(`Scenario: Visiting a blank form for ${model}`, async function (assert, nspace) {
      createModels('dc', 1, 'datacenter');

      await visit(model, { dc: 'datacenter' }, { nspace });

      assert.strictEqual(currentURL(), nspaceURL(nspace, `/datacenter/${slug}/create`));
      assert.notOk(page().submitIsEnabled, 'the submit button is disabled');
      assert.strictEqual(currentURL(), nspaceURL(nspace, `/datacenter/${slug}/create`));
    });
  });
});
