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
  assertVisibleCount,
} from 'consul-ui/tests/helpers/acceptance';

module('Acceptance | dc / acls / policies / as many / list: List', function (hooks) {
  setupAcceptanceTest(hooks);

  ['token', 'role'].forEach((model) => {
    nspaceScenario(
      `Scenario: Listing policies as children of the ${model} page`,
      async function (assert, nspace) {
        createModels('dc', 1, 'datacenter');
        createModels(model, 1, {
          ServiceIdentities: [{ ServiceName: 'Service-Identity' }],
          NodeIdentities: [{ NodeName: 'Node-Identity' }],
          Policies: [
            { Name: 'Policy', ID: 0 },
            { Name: 'Policy 2', ID: 2 },
            { Name: 'Policy 3', ID: 3 },
          ],
        });

        await visit(model, { dc: 'datacenter', [model]: 'key' }, { nspace });
        assert.equal(currentURL(), nspaceURL(nspace, `/datacenter/acls/${model}s/key`));

        // Identities turn into policies
        assertVisibleCount(assert, page().policies.selectedOptions, 5, 'policy models');
      }
    );
  });
});
