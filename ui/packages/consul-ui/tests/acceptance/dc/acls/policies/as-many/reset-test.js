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
  fillInForm,
} from 'consul-ui/tests/helpers/acceptance';

module('Acceptance | dc / acls / policies / as many / reset: Reset policy form', function (hooks) {
  setupAcceptanceTest(hooks);

  ['token', 'role'].forEach((model) => {
    nspaceScenario(
      `Scenario: Adding a new policy as a child of ${model}`,
      async function (assert, nspace) {
        createModels('dc', 1, 'datacenter');
        createModels(model, 1, { Policies: null, ServiceIdentities: null });

        await visit(model, { dc: 'datacenter', [model]: 'key' }, { nspace });
        assert.equal(currentURL(), nspaceURL(nspace, `/datacenter/acls/${model}s/key`));
        await page().policies.create();

        await fillInForm('policy', {
          Name: 'New-Policy',
          Description: 'New Policy Description',
          Rules: 'operator {}',
        });
        await page().policies.form.cancel();
        await page().policies.create();

        // The form is empty again (inputs by name, the Rules editor by aria-label).
        ['Name', 'Description', 'Rules'].forEach((key) => {
          const name = `policy[${key}]`;
          const element =
            document.querySelector(`[name="${name}"]`) ||
            document.querySelector(`[aria-label="${name}"]`);
          assert.strictEqual(element.value || element.textContent, '', `${name} is empty`);
        });
      }
    );
  });
});
