/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

// Native QUnit port of tests/acceptance/components/copy-button.feature.
//
// The original feature carried a feature-level `@ignore`, so the runner skipped
// it. We preserve that here with `{ ignore: true }` (which maps to QUnit's
// `skip`) rather than deleting the coverage outright.

import { module } from 'qunit';
import { click } from '@ember/test-helpers';

import {
  setupAcceptanceTest,
  nspaceScenario,
  nspaceURL,
  api,
  visit,
  currentURL,
  clipboard,
} from 'consul-ui/tests/helpers/acceptance';

module('Acceptance | components / copy-button', function (hooks) {
  setupAcceptanceTest(hooks);

  nspaceScenario(
    'Clicking the copy button',
    async function (assert, nspace) {
      api.server.createList('dc', 1, 'dc-1');
      api.server.createList('node', 1, {
        ID: 'node-0',
        Checks: [
          {
            Name: 'gprc-check',
            Node: 'node-0',
            CheckID: 'grpc-check',
            Status: 'passing',
            Type: 'grpc',
            Output: 'The output',
            Notes: 'The notes',
          },
        ],
      });

      await visit('node', { dc: 'dc-1', node: 'node-0' }, { nspace });
      assert.equal(currentURL(), nspaceURL(nspace, '/dc-1/nodes/node-0/health-checks'));

      await click('.healthcheck-output:nth-child(1) .copy-button button');

      assert.ok(clipboard().includes('The output'), 'copied "The output" to the clipboard');
    },
    { ignore: true }
  );
});
