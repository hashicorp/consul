/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

// Native QUnit port of tests/acceptance/dc/acls/tokens/update.feature.
//
// The `.feature` titled its Where-table scenario "Update to [Name]", but the
// table only had a Description column, so both rows ran as "Update to [Name]".
// The native titles use the description instead.

import { module } from 'qunit';
import { fillIn } from '@ember/test-helpers';

import {
  setupAcceptanceTest,
  nspaceScenario,
  nspaceURL,
  nspaceEndpoint,
  createModels,
  respondWith,
  visit,
  submit,
  currentURL,
  requestMade,
  requestMadeWithoutProperties,
} from 'consul-ui/tests/helpers/acceptance';

// Background: the edit form for the "key" token.
const visitEdit = async function (assert, nspace) {
  createModels('dc', 1, 'datacenter');
  createModels('token', 1, { AccessorID: 'key', Policies: null });

  await visit('token', { dc: 'datacenter', token: 'key' }, { nspace });

  assert.equal(currentURL(), nspaceURL(nspace, '/datacenter/acls/tokens/key'));
  assert.equal(document.title, 'Edit Token - Consul');
};

module('Acceptance | dc / acls / tokens / update: ACL Token Update', function (hooks) {
  setupAcceptanceTest(hooks);

  ['description', 'description with spaces'].forEach((description) => {
    nspaceScenario(`Scenario: Update to ${description}`, async function (assert, nspace) {
      await visitEdit(assert, nspace);

      await fillIn('[name="Description"]', description);
      await submit();

      requestMade(
        assert,
        'PUT',
        nspaceEndpoint('/v1/acl/token/key?dc=datacenter&ns=@!namespace', nspace),
        { body: { Description: description } }
      );
      assert.equal(currentURL(), nspaceURL(nspace, '/datacenter/acls/tokens'));
      assert.dom('[data-notification]').hasClass('hds-toast');
      assert.dom('[data-notification]').hasClass('hds-alert--color-success');
    });
  });

  nspaceScenario('Scenario: There was an error saving the key', async function (assert, nspace) {
    await visitEdit(assert, nspace);

    respondWith('/v1/acl/token/key', { status: 500 });
    await submit();

    assert.equal(currentURL(), nspaceURL(nspace, '/datacenter/acls/tokens/key'));
    assert.dom('[data-notification]').hasClass('hds-toast');
    assert.dom('[data-notification]').hasClass('hds-alert--color-critical');
  });

  nspaceScenario(
    'Scenario: Updating a simple ACL token when Namespaces are disabled does not send Namespace',
    async function (assert, nspace) {
      await visitEdit(assert, nspace);

      await fillIn('[name="Description"]', 'Description');
      await submit();

      requestMadeWithoutProperties(assert, 'PUT', '/v1/acl/token/key?dc=datacenter', ['Namespace']);
      assert.equal(currentURL(), nspaceURL(nspace, '/datacenter/acls/tokens'));
      assert.dom('[data-notification]').hasClass('hds-toast');
      assert.dom('[data-notification]').hasClass('hds-alert--color-success');
    },
    { notNamespaceable: true }
  );
});
