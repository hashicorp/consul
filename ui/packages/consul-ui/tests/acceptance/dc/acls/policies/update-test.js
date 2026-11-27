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
  respondWith,
  visit,
  page,
  submit,
  currentURL,
  fillInForm,
  assertVisibleCount,
  requestMade,
  requestMadeWithoutProperties,
} from 'consul-ui/tests/helpers/acceptance';

// Background: the edit form for the "policy-id" policy, used by 3 tokens.
const visitEdit = async function (assert, nspace) {
  createModels('dc', 1, 'datacenter');
  createModels('policy', 1, { ID: 'policy-id', Datacenters: [] });
  createModels('token', 3);

  await visit('policy', { dc: 'datacenter', policy: 'policy-id' }, { nspace });

  assert.equal(currentURL(), nspaceURL(nspace, '/datacenter/acls/policies/policy-id'));
  assertVisibleCount(assert, page().tokens, 3, 'token models');
  assert.equal(document.title, 'Edit Policy - Consul');
};

const rows = [
  {
    Name: 'policy-name',
    Rules: 'key "foo" {policy = "read"}',
    Description: 'policy-name description',
  },
  {
    Name: 'policy_name',
    Rules: 'key "foo" {policy = "write"}',
    Description: 'policy name description',
  },
  {
    Name: 'policyName',
    Rules: 'key "foo" {policy = "read"}',
    Description: 'policy%20name description',
  },
];

module('Acceptance | dc / acls / policies / update: ACL Policy Update', function (hooks) {
  setupAcceptanceTest(hooks);

  rows.forEach(({ Name, Rules, Description }) => {
    nspaceScenario(
      `Scenario: Update to ${Name}, ${Rules}, ${Description}`,
      async function (assert, nspace) {
        await visitEdit(assert, nspace);

        await fillInForm('policy', { Name, Description, Rules });
        await page().validDatacenters();
        await page().datacenter();
        await submit();

        requestMade(
          assert,
          'PUT',
          nspaceEndpoint('/v1/acl/policy/policy-id?dc=datacenter&ns=@!namespace', nspace),
          { body: { Name, Description, Rules, Datacenters: ['datacenter'] } }
        );
        assert.equal(currentURL(), nspaceURL(nspace, '/datacenter/acls/policies'));
        assert.dom('[data-notification]').hasClass('hds-toast');
        assert.dom('[data-notification]').hasClass('hds-alert--color-success');
      }
    );
  });

  nspaceScenario('Scenario: There was an error saving the key', async function (assert, nspace) {
    await visitEdit(assert, nspace);

    respondWith('/v1/acl/policy/policy-id', { status: 500 });
    await submit();

    assert.equal(currentURL(), nspaceURL(nspace, '/datacenter/acls/policies/policy-id'));
    assert.dom('[data-notification]').hasClass('hds-toast');
    assert.dom('[data-notification]').hasClass('hds-alert--color-critical');
  });

  nspaceScenario(
    'Scenario: Updating a simple ACL policy when Namespaces are disabled does not send Namespace',
    async function (assert, nspace) {
      await visitEdit(assert, nspace);

      await fillInForm('policy', { Description: 'Description' });
      await submit();

      requestMadeWithoutProperties(assert, 'PUT', '/v1/acl/policy/policy-id?dc=datacenter', [
        'Namespace',
      ]);
      assert.equal(currentURL(), nspaceURL(nspace, '/datacenter/acls/policies'));
      assert.dom('[data-notification]').hasClass('hds-toast');
      assert.dom('[data-notification]').hasClass('hds-alert--color-success');
    },
    { notNamespaceable: true }
  );
});
