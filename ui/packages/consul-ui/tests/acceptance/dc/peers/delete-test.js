/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

// Native QUnit port of tests/acceptance/dc/peers/delete.feature.

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

const deletePeer = async function () {
  const peer = page().peers.objectAt(0);
  await peer.actions();
  await peer.delete();
  await peer.confirmDelete();
};

module(
  'Acceptance | dc / peers / delete: Deleting items with confirmations, success and error notifications',
  function (hooks) {
    setupAcceptanceTest(hooks);

    hooks.beforeEach(function () {
      createModels('dc', 1, 'datacenter');
    });

    nspaceScenario(
      'Scenario: Deleting a peer model from the listing page',
      async function (assert, nspace) {
        createModels('peer', 1, { Name: 'peer-name', State: 'ACTIVE' });

        await visit('peers', { dc: 'datacenter' }, { nspace });
        await deletePeer();

        requestMade(assert, 'DELETE', '/v1/peering/peer-name');
        assert.dom('[data-notification]').hasClass('hds-toast');
        assert.dom('[data-notification]').hasClass('hds-alert--color-success');
      }
    );

    nspaceScenario(
      'Scenario: Deleting a peer from the peer listing page with error',
      async function (assert, nspace) {
        createModels('peer', 1, { Name: 'peer-name', State: 'ACTIVE' });

        await visit('peers', { dc: 'datacenter' }, { nspace });
        respondWith('/v1/peering/peer-name', { status: 500 });
        await deletePeer();

        assert.dom('[data-notification]').hasClass('hds-toast');
        assert.dom('[data-notification]').hasClass('hds-alert--color-critical');
      }
    );

    nspaceScenario(
      'Scenario: A Peer currently deleting cannot be deleted',
      async function (assert, nspace) {
        createModels('peer', 1, { Name: 'peer-name', State: 'DELETING' });

        await visit('peers', { dc: 'datacenter' }, { nspace });
        assert
          .dom('.consul-peer-list [data-test-tabular-row] [data-test-actions-menu]')
          .doesNotExist('a deleting peer has no actions');
      }
    );
  }
);
