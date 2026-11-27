/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module } from 'qunit';

import {
  setupAcceptanceTest,
  nspaceScenario,
  nspaceURL,
  nspaceEndpoint,
  createModels,
  visit,
  currentURL,
  requestMade,
} from 'consul-ui/tests/helpers/acceptance';

module('Acceptance | dc / kvs / trailing-slash', function (hooks) {
  setupAcceptanceTest(hooks);

  [
    ['without', 'foo/bar'],
    ['with', 'foo/bar/'],
  ].forEach(([withOrWithout, kv]) => {
    nspaceScenario(
      `Scenario: I have 10 folders and I visit ${withOrWithout} a trailing slash`,
      async function (assert, nspace) {
        createModels('dc', 1, 'datacenter');
        createModels('kv', 10);

        await visit('kvs', { dc: 'datacenter', kv }, { nspace });
        assert.equal(currentURL(), nspaceURL(nspace, '/datacenter/kv/foo/bar/'));
        requestMade(
          assert,
          'GET',
          nspaceEndpoint('/v1/kv/foo/bar/?keys&dc=datacenter&separator=%2F&ns=@namespace', nspace)
        );
      }
    );
  });
});
