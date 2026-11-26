/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

// Native QUnit port of tests/acceptance/dc/intentions/update.feature.

import { module } from 'qunit';
import { click, fillIn } from '@ember/test-helpers';

import {
  setupAcceptanceTest,
  nspaceScenario,
  nspaceURL,
  createModels,
  respondWith,
  visit,
  submit,
  currentURL,
  requestMade,
} from 'consul-ui/tests/helpers/acceptance';

const EXACT =
  '/v1/connect/intentions/exact?source=default%2Fdefault%2Fweb&destination=default%2Fdefault%2Fdb&dc=datacenter';

// Background: the edit form for the "intention-id" intention.
const visitEdit = async function (assert, nspace) {
  createModels('dc', 1, 'datacenter');
  createModels('intention', 1, {
    SourceName: 'web',
    DestinationName: 'db',
    SourceNS: 'default',
    DestinationNS: 'default',
    SourcePartition: 'default',
    DestinationPartition: 'default',
    ID: 'intention-id',
  });

  await visit('intention', { dc: 'datacenter', intention: 'intention-id' }, { nspace });
  assert.equal(currentURL(), nspaceURL(nspace, '/datacenter/intentions/intention-id'));
  assert.equal(document.title, 'Edit Intention - Consul');
};

module('Acceptance | dc / intentions / update: Intention Update', function (hooks) {
  setupAcceptanceTest(hooks);

  [{ Description: 'Desc', Action: 'allow' }].forEach(({ Description, Action }) => {
    nspaceScenario(
      `Scenario: Update to ${Description}, ${Action}`,
      async function (assert, nspace) {
        await visitEdit(assert, nspace);

        await fillIn('[name="Description"]', Description);
        await click(`[value=${Action}]`);
        await submit();

        requestMade(assert, 'PUT', EXACT, { body: { Description, Action } });
        assert.equal(currentURL(), nspaceURL(nspace, '/datacenter/intentions'));
        assert.equal(document.title, 'Intentions - Consul');
        assert.dom('[data-notification]').hasClass('hds-toast');
        assert.dom('[data-notification]').hasClass('hds-alert--color-success');
      }
    );
  });

  nspaceScenario(
    'Scenario: There was an error saving the intention',
    async function (assert, nspace) {
      await visitEdit(assert, nspace);

      respondWith(EXACT, { status: 500 });
      await submit();

      assert.equal(currentURL(), nspaceURL(nspace, '/datacenter/intentions/intention-id'));
      assert.dom('[data-notification]').hasClass('hds-toast');
      assert.dom('[data-notification]').hasClass('hds-alert--color-critical');
    }
  );
});
