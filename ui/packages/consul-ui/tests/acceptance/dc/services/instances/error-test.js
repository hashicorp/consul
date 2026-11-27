/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

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
  "Acceptance | dc / services / instances / error: Visit Service Instance what doesn't exist",
  function (hooks) {
    setupAcceptanceTest(hooks);

    nspaceScenario(
      'Scenario: No instance can be found in the API response',
      async function (assert, nspace) {
        createModels('dc', 1, 'dc1');
        createModels('instance', 1);

        await visit(
          'instance',
          { dc: 'dc1', service: 'service-0', node: 'node-0', id: 'id-that-doesnt-exist' },
          { nspace }
        );
        assert.equal(
          currentURL(),
          nspaceURL(
            nspace,
            '/dc1/services/service-0/instances/node-0/id-that-doesnt-exist/health-checks'
          )
        );
        assert.strictEqual(page().error.status, '404');
      }
    );
  }
);
