/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

// Native QUnit port of tests/acceptance/dc/acls/policies/view-read-only.feature.
// The feature had a single untitled scenario (reported as "Scenario: "), so the
// native test is given a title.

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

const ID = '00000000-0000-0000-0000-000000000002';

module(
  'Acceptance | dc / acls / policies / view read-only policy: Readonly management policy',
  function (hooks) {
    setupAcceptanceTest(hooks);

    hooks.beforeEach(function () {
      createModels('dc', 1, 'datacenter');
      createModels('policy', 1, { ID });
    });

    nspaceScenario(
      'Scenario: The builtin read-only policy is read-only',
      async function (assert, nspace) {
        await visit('policy', { dc: 'datacenter', policy: ID }, { nspace });

        assert.equal(currentURL(), nspaceURL(nspace, `/datacenter/acls/policies/${ID}`));
        assert.dom('h1').includesText('View Policy');
        assert.dom('main form > div [data-test-delete]').doesNotExist('there is no delete button');
        assert.dom('main form > div [type=reset]').doesNotExist('there is no cancel button');
        // The yadda `I see tokens` step only checked that the page has a token list.
        assert.ok(page().tokens, 'the page has a token list');
      }
    );
  }
);
