/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

// Native QUnit port of tests/acceptance/dc/acls/policies/as-many/add-new.feature.
//
// The whole feature was `@ignore`d (yadda reported it as a single skipped
// test). Each scenario and `Where:` row is ported as its own test, and all of
// them run again except the identity ones (see IDENTITY_SAVE_FAILS). The two
// "Try to edit ... using the code editor" scenarios had the same title for
// every row, so the native titles add the model.

import { module } from 'qunit';

import {
  setupAcceptanceTest,
  nspaceScenario,
  nspaceURL,
  nspaceEndpoint,
  createModels,
  respondWith,
  visit,
  page,
  submit,
  currentURL,
  fillInForm,
  fillInCodeEditor,
  requestMade,
} from 'consul-ui/tests/helpers/acceptance';

const models = ['token', 'role'];

// Background: the model's edit page with the new-policy form open.
const openNewPolicyForm = async function (assert, model, nspace) {
  createModels('dc', 1, 'datacenter');
  createModels(model, 1, { Policies: null, ServiceIdentities: null, NodeIdentities: null });

  await visit(model, { dc: 'datacenter', [model]: 'key' }, { nspace });
  assert.equal(currentURL(), nspaceURL(nspace, `/datacenter/acls/${model}s/key`));
  await page().policies.create();
};

const assertSaved = function (assert, model, nspace, body) {
  requestMade(
    assert,
    'PUT',
    nspaceEndpoint(`/v1/acl/${model}/key?dc=datacenter&ns=@!namespace`, nspace),
    { body }
  );
  assert.equal(currentURL(), nspaceURL(nspace, `/datacenter/acls/${model}s`));
  assert.dom('[data-notification]').hasClass('hds-toast');
  assert.dom('[data-notification]').hasClass('hds-alert--color-success');
};

// FIXME: after adding a new service or node identity in the modal, saving the
// token/role shows an error toast and sends no PUT, so these stay skipped.
const IDENTITY_SAVE_FAILS = { ignore: true };

const newPolicy = {
  Name: 'New-Policy',
  Description: 'New Policy Description',
  Rules: 'key {}',
};

module('Acceptance | dc / acls / policies / as many / add new: Add new policy', function (hooks) {
  setupAcceptanceTest(hooks);

  models.forEach((model) => {
    nspaceScenario(
      `Scenario: Adding a new policy as a child of ${model}`,
      async function (assert, nspace) {
        await openNewPolicyForm(assert, model, nspace);

        await fillInForm('policy', newPolicy);
        await page().policies.form.submit();
        requestMade(
          assert,
          'PUT',
          nspaceEndpoint('/v1/acl/policy?dc=datacenter&ns=@namespace', nspace),
          { body: newPolicy }
        );

        await submit();
        assertSaved(assert, model, nspace, {
          Policies: [{ ID: 'ee52203d-989f-4f7a-ab5a-2bef004164ca-1', Name: 'New-Policy' }],
        });
      }
    );
  });

  models.forEach((model) => {
    nspaceScenario(
      `Scenario: Adding a new service identity as a child of ${model}`,
      async function (assert, nspace) {
        await openNewPolicyForm(assert, model, nspace);

        await fillInForm('policy', { Name: 'New-Service-Identity' });
        await page().policies.form.serviceIdentity();
        await page().policies.form.submit();
        await submit();

        assertSaved(assert, model, nspace, {
          ServiceIdentities: [{ ServiceName: 'New-Service-Identity' }],
        });
      },
      IDENTITY_SAVE_FAILS
    );
  });

  models.forEach((model) => {
    nspaceScenario(
      `Scenario: Adding a new node identity as a child of ${model}`,
      async function (assert, nspace) {
        await openNewPolicyForm(assert, model, nspace);

        await fillInForm('policy', { Name: 'New-Node-Identity' });
        await page().policies.form.nodeIdentity();
        await page().policies.form.submit();
        await submit();

        assertSaved(assert, model, nspace, {
          NodeIdentities: [{ NodeName: 'New-Node-Identity', Datacenter: 'datacenter' }],
        });
      },
      IDENTITY_SAVE_FAILS
    );
  });

  models.forEach((model) => {
    nspaceScenario(
      `Scenario: Adding a new policy as a child of ${model} and getting an error`,
      async function (assert, nspace) {
        const error =
          'Invalid service policy: acl.ServicePolicy{Name:"service", Policy:"", Sentinel:acl.Sentinel{Code:"", EnforcementLevel:""}, Intentions:""}';
        respondWith('/v1/acl/policy', { status: 500, body: `${error}\n` });
        await openNewPolicyForm(assert, model, nspace);

        await fillInForm('policy', newPolicy);
        await page().policies.form.submit();

        requestMade(
          assert,
          'PUT',
          nspaceEndpoint('/v1/acl/policy?dc=datacenter&ns=@namespace', nspace),
          { body: newPolicy }
        );
        assert.strictEqual(page().policies.form.rules.error, error);
      }
    );
  });

  models.forEach((model) => {
    nspaceScenario(
      `Scenario: Try to edit the Service Identity using the code editor on the ${model} page`,
      async function (assert, nspace) {
        await openNewPolicyForm(assert, model, nspace);

        await page().policies.form.serviceIdentity();
        await assert.rejects(
          fillInCodeEditor('policy[Rules]', 'key {}'),
          'the rules are read-only for a service identity'
        );
      }
    );
  });

  models.forEach((model) => {
    nspaceScenario(
      `Scenario: Try to edit the Node Identity using the code editor on the ${model} page`,
      async function (assert, nspace) {
        await openNewPolicyForm(assert, model, nspace);

        await page().policies.form.nodeIdentity();
        await assert.rejects(
          fillInCodeEditor('policy[Rules]', 'key {}'),
          'the rules are read-only for a node identity'
        );
      }
    );
  });
});
