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
  createModels,
  respondWith,
  visit,
  page,
  submit,
  currentURL,
  requestMade,
} from 'consul-ui/tests/helpers/acceptance';

module('Acceptance | dc / peers / create: Peer Create Token', function (hooks) {
  setupAcceptanceTest(hooks);

  hooks.beforeEach(function () {
    createModels('dc', 1, 'dc-1');
  });

  nspaceScenario(
    'Scenario: Visiting the Add peer connection page directly',
    async function (assert, nspace) {
      await visit('peers', { dc: 'dc-1' }, { nspace });
      assert.equal(currentURL(), nspaceURL(nspace, '/dc-1/peers'));

      await page().create();
      assert.equal(currentURL(), nspaceURL(nspace, '/dc-1/peers/create'));
      assert.equal(document.title, 'Add peer connection - Consul');
    }
  );

  nspaceScenario(
    'Scenario: Generating a token from the Generate token tab',
    async function (assert, nspace) {
      respondWith('/v1/peering/token', { body: { PeeringToken: 'an-encoded-token' } });

      await visit('peerCreate', { dc: 'dc-1' }, { nspace });
      await fillIn('[name="Name"]', 'new-peer');
      await submit();

      requestMade(assert, 'POST', '/v1/peering/token', {
        body: { PeerName: 'new-peer', ServerExternalAddresses: [] },
      });
      assert.dom('#copy-token-modal [data-test-peering-token]').includesText('an-encoded-token');
    }
  );

  nspaceScenario(
    'Scenario: Establishing a peering from the Establish peering tab',
    async function (assert, nspace) {
      respondWith('/v1/peering/establish', { body: { Status: 200 } });

      await visit('peerCreate', { dc: 'dc-1' }, { nspace });
      await page().tabs.initiate();
      await fillIn('[name="Name"]', 'new-peer');
      await fillIn('[name="PeeringToken"]', 'an-encoded-token');
      await submit();

      requestMade(assert, 'POST', '/v1/peering/establish', {
        body: { PeerName: 'new-peer', PeeringToken: 'an-encoded-token' },
      });
      assert.equal(currentURL(), nspaceURL(nspace, '/dc-1/peers/new-peer/imported-services'));
    }
  );

  nspaceScenario(
    'Scenario: Cancelling with unsaved input asks for confirmation',
    async function (assert, nspace) {
      await visit('peerCreate', { dc: 'dc-1' }, { nspace });
      await fillIn('[name="Name"]', 'new-peer');

      await page().cancel();
      assert.equal(currentURL(), nspaceURL(nspace, '/dc-1/peers/create'));
      await page().keepEditing();
      assert.equal(currentURL(), nspaceURL(nspace, '/dc-1/peers/create'));

      await page().cancel();
      await page().confirmDiscard();
      assert.equal(currentURL(), nspaceURL(nspace, '/dc-1/peers'));
    }
  );

  nspaceScenario(
    'Scenario: Cancelling an empty form does not ask for confirmation',
    async function (assert, nspace) {
      await visit('peerCreate', { dc: 'dc-1' }, { nspace });

      await page().cancel();
      assert.equal(currentURL(), nspaceURL(nspace, '/dc-1/peers'));
    }
  );
});
