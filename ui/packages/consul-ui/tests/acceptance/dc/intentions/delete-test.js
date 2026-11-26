/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

// Native QUnit port of tests/acceptance/dc/intentions/delete.feature.

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

const ID = 'ee52203d-989f-4f7a-ab5a-2bef004164ca';
const EXACT =
  '/v1/connect/intentions/exact?source=default%2Fdefault%2Fname&destination=default%2Fdefault%2Fdestination&dc=datacenter';

module(
  'Acceptance | dc / intentions / deleting: Deleting items with confirmations, success and error notifications',
  function (hooks) {
    setupAcceptanceTest(hooks);

    hooks.beforeEach(function () {
      createModels('dc', 1, 'datacenter');
      createModels('intention', 1, {
        SourcePeer: '',
        SourceName: 'name',
        DestinationName: 'destination',
        SourceNS: 'default',
        DestinationNS: 'default',
        SourcePartition: 'default',
        DestinationPartition: 'default',
        ID,
        Meta: null,
      });
    });

    nspaceScenario(
      'Scenario: Deleting a intention model from the intention listing page',
      async function (assert, nspace) {
        await visit('intentions', { dc: 'datacenter' }, { nspace });
        const intention = page().intentionList.intentions.objectAt(0);
        await intention.actions();
        await intention.delete();
        await intention.confirmInlineDelete();

        requestMade(assert, 'DELETE', EXACT);
        assert.dom('[data-notification]').hasClass('hds-toast');
        assert.dom('[data-notification]').hasClass('hds-alert--color-success');
      }
    );

    nspaceScenario(
      'Scenario: Deleting an intention from the intention detail page',
      async function (assert, nspace) {
        await visit('intention', { dc: 'datacenter', intention: ID }, { nspace });
        await page().delete();
        await page().confirmDelete();

        requestMade(assert, 'DELETE', EXACT);
        assert.dom('[data-notification]').hasClass('hds-toast');
        assert.dom('[data-notification]').hasClass('hds-alert--color-success');
      }
    );

    nspaceScenario(
      'Scenario: Deleting an intention from the intention detail page and getting an error',
      async function (assert, nspace) {
        await visit('intention', { dc: 'datacenter', intention: ID }, { nspace });
        respondWith(EXACT, { status: 500 });
        await page().delete();
        await page().confirmDelete();

        assert.dom('[data-notification]').hasClass('hds-toast');
        assert.dom('[data-notification]').hasClass('hds-alert--color-critical');
      }
    );

    nspaceScenario(
      'Scenario: Deleting an intention from the intention detail page and getting an error due to a duplicate intention',
      async function (assert, nspace) {
        await visit('intention', { dc: 'datacenter', intention: ID }, { nspace });
        respondWith(EXACT, { status: 500, body: 'duplicate intention found:' });
        await page().delete();
        await page().confirmDelete();

        assert.dom('[data-notification]').hasClass('hds-toast');
        assert.dom('[data-notification]').hasClass('hds-alert--color-critical');
      }
    );
  }
);
