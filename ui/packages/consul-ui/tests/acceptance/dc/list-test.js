/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

// Native QUnit port of tests/acceptance/dc/list.feature.
//
// The original scenario outline listed one row per model type; here we
// parameterise the scenario body over those rows.

import { module } from 'qunit';

import {
  setupAcceptanceTest,
  nspaceScenario,
  nspaceURL,
  api,
  visit,
  page,
  currentURL,
} from 'consul-ui/tests/helpers/acceptance';

const visible = (collection) => collection.filter((item) => item.isVisible).length;

// Model | Page | Url | page-object collection field
const cases = [
  { model: 'node', pageName: 'nodes', url: '/dc-1/nodes', field: 'nodes' },
  { model: 'kv', pageName: 'kvs', url: '/dc-1/kv', field: 'kvs' },
  { model: 'token', pageName: 'tokens', url: '/dc-1/acls/tokens', field: 'tokens' },
  { model: 'policy', pageName: 'policies', url: '/dc-1/acls/policies', field: 'policies' },
];

module('Acceptance | dc / list: List Models', function (hooks) {
  setupAcceptanceTest(hooks);

  cases.forEach(({ model, pageName, url, field }) => {
    nspaceScenario(`Listing ${model}`, async function (assert, nspace) {
      api.server.createList('dc', 1, 'dc-1');
      api.server.createList(model, 3);

      await visit(pageName, { dc: 'dc-1' }, { nspace });
      assert.equal(currentURL(), nspaceURL(nspace, url));

      assert.equal(visible(page()[field]), 3, `sees 3 ${model} models`);
    });
  });
});
