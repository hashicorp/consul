/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

// Native QUnit port of tests/acceptance/components/kv-filter.feature.
//
// The `.feature` used a scenario outline (Where table) with one row per search
// term; here we parameterise the same scenario body over those terms.

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

module('Acceptance | components / kv-filter', function (hooks) {
  setupAcceptanceTest(hooks);

  ['hi', 'there'].forEach((text) => {
    nspaceScenario(
      `Filtering using the freetext filter with ${text}`,
      async function (assert, nspace) {
        api.server.createList('dc', 1, 'dc-1');
        api.server.createList('kv', 2, ['hi', 'there']);

        await visit('kvs', { dc: 'dc-1' }, { nspace });
        assert.equal(currentURL(), nspaceURL(nspace, '/dc-1/kv'));

        await fillIn('[name="s"]', text);

        // mirrors `I see 1 kv model with the name "<Text>"`
        const matching = [...page().kvs].filter((item) => item.isVisible && item.name === text);
        assert.equal(matching.length, 1, `Expected 1 kv named "${text}", saw ${matching.length}`);
      }
    );
  });
});
