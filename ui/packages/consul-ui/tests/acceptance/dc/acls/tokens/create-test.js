/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module } from 'qunit';
import { fillIn } from '@ember/test-helpers';

import {
  setupAcceptanceTest,
  nspaceScenario,
  nspaceURL,
  nspaceEndpoint,
  createModels,
  visit,
  submit,
  currentURL,
  requestMade,
  requestMadeWithoutProperties,
} from 'consul-ui/tests/helpers/acceptance';

// Background: the create form for the "datacenter" datacenter.
const visitCreate = function (nspace) {
  createModels('dc', 1, 'datacenter');
  return visit('token', { dc: 'datacenter' }, { nspace });
};

module('Acceptance | dc / acls / tokens / create', function (hooks) {
  setupAcceptanceTest(hooks);

  nspaceScenario(
    'Scenario: Visiting the page without error and the title is correct',
    async function (assert, nspace) {
      await visitCreate(nspace);

      assert.equal(currentURL(), nspaceURL(nspace, '/datacenter/acls/tokens/create'));
      assert.equal(document.title, 'New Token - Consul');
    }
  );

  ['description', 'description with spaces'].forEach((description) => {
    nspaceScenario(
      `Scenario: Creating a simple ACL token with description ${description}`,
      async function (assert, nspace) {
        await visitCreate(nspace);

        await fillIn('[name="Description"]', description);
        await submit();

        requestMade(
          assert,
          'PUT',
          nspaceEndpoint('/v1/acl/token?dc=datacenter&ns=@namespace', nspace),
          { body: { Description: description } }
        );
        assert.equal(currentURL(), nspaceURL(nspace, '/datacenter/acls/tokens'));
        assert.dom('[data-notification]').hasClass('hds-toast');
        assert.dom('[data-notification]').hasClass('hds-alert--color-success');
      }
    );
  });

  nspaceScenario(
    'Scenario: Creating a simple ACL token when Namespaces are disabled does not send Namespace',
    async function (assert, nspace) {
      await visitCreate(nspace);

      await fillIn('[name="Description"]', 'Description');
      await submit();

      requestMadeWithoutProperties(assert, 'PUT', '/v1/acl/token?dc=datacenter', ['Namespace']);
      assert.equal(currentURL(), nspaceURL(nspace, '/datacenter/acls/tokens'));
      assert.dom('[data-notification]').hasClass('hds-toast');
      assert.dom('[data-notification]').hasClass('hds-alert--color-success');
    },
    { notNamespaceable: true }
  );
});
