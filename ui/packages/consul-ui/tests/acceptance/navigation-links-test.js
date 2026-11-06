/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

// Native QUnit port of tests/acceptance/navigation-links.feature

import { module } from 'qunit';

import {
  setupAcceptanceTest,
  nspaceScenario,
  createModels,
  respondWith,
  visit,
  currentURL,
  nspaceURL,
} from 'consul-ui/tests/helpers/acceptance';

const navItem = (name) => `[data-test-navigation] [data-test-main-nav-${name}] > *`;

module('Acceptance | navigation-links: Main Navigation link visibility', function (hooks) {
  setupAcceptanceTest(hooks);

  nspaceScenario('Scenario: No read access to Key/Values', async function (assert, nspace) {
    createModels('dc', 1, 'dc-1');
    respondWith('/v1/internal/acl/authorize', {
      body: [
        { Resource: 'operator', Access: 'write', Allow: true },
        { Resource: 'service', Access: 'read', Allow: true },
        { Resource: 'node', Access: 'read', Allow: true },
        { Resource: 'key', Access: 'read', Allow: true },
        { Resource: 'intention', Access: 'read', Allow: true },
        { Resource: 'acl', Access: 'read', Allow: false },
      ],
    });

    await visit('services', { dc: 'dc-1' }, { nspace });

    assert.dom(navItem('services')).exists('the services link is shown');
    assert.dom(navItem('roles')).doesNotExist('the roles link is hidden without acl read access');
  });

  nspaceScenario('Scenario: Empty state login button is shown', async function (assert, nspace) {
    createModels('dc', 1, 'dc-1');
    createModels('service', 0);

    await visit('services', { dc: 'dc-1' }, { nspace });

    assert.strictEqual(currentURL(), nspaceURL(nspace, '/dc-1/services'));
    assert.dom('.empty-state [data-test-empty-state-login]').exists();
  });
});
