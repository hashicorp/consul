/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module } from 'qunit';

import {
  setupAcceptanceTest,
  nspaceScenario,
  nspaceEndpoint,
  createModels,
  respondWith,
  visit,
  page,
  requestMade,
} from 'consul-ui/tests/helpers/acceptance';

const DELETE_URL = '/v1/kv/key-name?dc=datacenter&ns=@!namespace';

module(
  'Acceptance | dc / kvs / deleting: Deleting items with confirmations, success and error notifications',
  function (hooks) {
    setupAcceptanceTest(hooks);

    hooks.beforeEach(function () {
      createModels('dc', 1, 'datacenter');
    });

    nspaceScenario(
      'Scenario: Deleting a kv model from the kv listing page',
      async function (assert, nspace) {
        createModels('kv', 1, ['key-name']);

        await visit('kvs', { dc: 'datacenter' }, { nspace });
        const kv = page().kvs.objectAt(0);
        await kv.actions();
        await kv.delete();
        await kv.confirmInlineDelete();

        requestMade(assert, 'DELETE', nspaceEndpoint(DELETE_URL, nspace));
        assert.dom('[data-notification]').hasClass('hds-toast');
        assert.dom('[data-notification]').hasClass('hds-alert--color-success');
      }
    );

    nspaceScenario(
      'Scenario: Deleting an kv from the kv detail page',
      async function (assert, nspace) {
        await visit('kv', { dc: 'datacenter', kv: 'key-name' }, { nspace });
        await page().delete();
        await page().confirmDelete();

        requestMade(assert, 'DELETE', nspaceEndpoint(DELETE_URL, nspace));
        assert.dom('[data-notification]').hasClass('hds-toast');
        assert.dom('[data-notification]').hasClass('hds-alert--color-success');
      }
    );

    nspaceScenario(
      'Scenario: Deleting an kv from the kv detail page and getting an error',
      async function (assert, nspace) {
        await visit('kv', { dc: 'datacenter', kv: 'key-name' }, { nspace });
        respondWith('/v1/kv/key-name', { status: 500 });
        await page().delete();
        await page().confirmDelete();

        assert.dom('[data-notification]').hasClass('hds-toast');
        assert.dom('[data-notification]').hasClass('hds-alert--color-critical');
      }
    );
  }
);
