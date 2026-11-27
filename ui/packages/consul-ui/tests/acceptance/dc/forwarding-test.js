/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

// Arriving at a datacenter with only the dc in the URL redirects to that dc's
// services overview. CE only (`notNamespaceable`).

import { module } from 'qunit';

import {
  setupAcceptanceTest,
  nspaceScenario,
  api,
  visit,
  currentURL,
} from 'consul-ui/tests/helpers/acceptance';

module('Acceptance | dc / forwarding', function (hooks) {
  setupAcceptanceTest(hooks);

  nspaceScenario(
    'Arriving at the datacenter index page with no other url info',
    async function (assert) {
      api.server.createList('dc', 1, 'datacenter');

      await visit('dcs', { dc: 'datacenter' });
      assert.equal(currentURL(), '/datacenter/services');
    },
    { notNamespaceable: true }
  );
});
