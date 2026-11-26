/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

// Native QUnit port of tests/acceptance/dc/nspaces/delete.feature.
// The feature was `@onlyNamespaceable` (Enterprise only), as before.

import { module } from 'qunit';

import {
  setupAcceptanceTest,
  nspaceScenario,
  createModels,
  respondWith,
  visit,
  page,
  requestMade,
} from 'consul-ui/tests/helpers/acceptance';

const URL = '/v1/namespace/a-namespace?dc=datacenter';

module(
  'Acceptance | dc / nspaces / delete: Deleting items with confirmations, success and error notifications',
  function (hooks) {
    setupAcceptanceTest(hooks);

    hooks.beforeEach(function () {
      createModels('dc', 1, 'datacenter');
    });

    nspaceScenario(
      'Scenario: Deleting a nspace model from the nspaces listing page',
      async function (assert, nspace) {
        createModels('nspace', 1, { Name: 'a-namespace' });

        await visit('nspaces', { dc: 'datacenter' }, { nspace });
        const item = page().nspaces.objectAt(0);
        await item.actions();
        await item.delete();
        await item.confirmDelete();

        requestMade(assert, 'DELETE', URL);
        assert.dom('[data-notification]').hasClass('hds-toast');
        assert.dom('[data-notification]').hasClass('hds-alert--color-success');
      },
      { onlyNamespaceable: true }
    );

    nspaceScenario(
      'Scenario: Deleting a nspace from the nspace detail page with success',
      async function (assert, nspace) {
        await visit('nspace', { dc: 'datacenter', namespace: 'a-namespace' }, { nspace });
        await page().delete();
        await page().confirmDelete();

        requestMade(assert, 'DELETE', URL);
        assert.dom('[data-notification]').hasClass('hds-toast');
        assert.dom('[data-notification]').hasClass('hds-alert--color-success');
      },
      { onlyNamespaceable: true }
    );

    nspaceScenario(
      'Scenario: Deleting a nspace from the nspace detail page with error',
      async function (assert, nspace) {
        await visit('nspace', { dc: 'datacenter', namespace: 'a-namespace' }, { nspace });
        respondWith(URL, { status: 500 });
        await page().delete();
        await page().confirmDelete();

        assert.dom('[data-notification]').hasClass('hds-toast');
        assert.dom('[data-notification]').hasClass('hds-alert--color-critical');
      },
      { onlyNamespaceable: true }
    );
  }
);
