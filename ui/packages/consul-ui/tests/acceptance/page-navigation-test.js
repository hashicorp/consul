/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

// Native QUnit port of tests/acceptance/page-navigation.feature.
// (the feature's empty `@ignore` placeholder scenarios were deleted)

import { module } from 'qunit';
import { click } from '@ember/test-helpers';

import {
  setupAcceptanceTest,
  nspaceScenario,
  api,
  visit,
  page,
  pages,
  currentURL,
  nspaceURL,
  nspaceEndpoint,
  requestMade,
  requestsIncluded,
} from 'consul-ui/tests/helpers/acceptance';

module('Acceptance | page-navigation', function (hooks) {
  setupAcceptanceTest(hooks);

  // The yadda Background created one datacenter, dc1, for every scenario.
  hooks.beforeEach(function () {
    api.server.createList('dc', 1, 'dc1');
  });

  nspaceScenario('Scenario: Visiting the index page', async function (assert, nspace) {
    await visit('index', { dc: 'dc1' }, { nspace });

    assert.strictEqual(currentURL(), nspaceURL(nspace, '/dc1/services'));
  });

  [
    { link: 'nodes', url: '/dc1/nodes', endpoint: '/v1/internal/ui/nodes?dc=dc1&ns=@namespace' },
    {
      link: 'tokens',
      url: '/dc1/acls/tokens',
      endpoint: '/v1/acl/tokens?dc=dc1&ns=@namespace',
    },
  ].forEach(({ link, url, endpoint }) => {
    nspaceScenario(
      `Scenario: Clicking ${link} in the navigation takes me to ${url}`,
      async function (assert, nspace) {
        await visit('services', { dc: 'dc1' }, { nspace });

        await page().navigation[link]();

        assert.strictEqual(currentURL(), nspaceURL(nspace, url));
        requestMade(assert, 'GET', nspaceEndpoint(endpoint, nspace));
      }
    );
  });

  nspaceScenario(
    'Scenario: The node detail page calls the correct API endpoints',
    async function (assert, nspace) {
      await visit('node', { dc: 'dc1', node: 'node-0' }, { nspace });

      assert.strictEqual(currentURL(), nspaceURL(nspace, '/dc1/nodes/node-0/health-checks'));
      requestsIncluded(
        assert,
        'GET',
        [
          '/v1/catalog/datacenters',
          '/v1/internal/ui/node/node-0?dc=dc1&ns=@namespace',
          '/v1/coordinate/nodes?dc=dc1',
        ].map((url) => nspaceEndpoint(url, nspace))
      );
    }
  );

  nspaceScenario(
    'Scenario: The policies page/tab calls the correct API endpoints',
    async function (assert, nspace) {
      await visit('policies', { dc: 'dc1' }, { nspace });

      assert.strictEqual(currentURL(), nspaceURL(nspace, '/dc1/acls/policies'));
      requestsIncluded(
        assert,
        'GET',
        ['/v1/catalog/datacenters', '/v1/acl/policies?dc=dc1&ns=@namespace'].map((url) =>
          nspaceEndpoint(url, nspace)
        )
      );
    }
  );

  // The scenarios below were `@ignore`d (FIXME) in the original feature and are
  // kept as skipped tests so the coverage gap stays visible.
  nspaceScenario(
    'Scenario: Clicking a kv in the kvs listing and back again',
    async function () {},
    { ignore: true }
  );

  nspaceScenario(
    'Scenario: The kv detail page calls the correct API endpoints',
    async function () {},
    { ignore: true }
  );

  nspaceScenario(
    'Scenario: Clicking a kv in the kvs listing and cancelling',
    async function () {},
    { ignore: true }
  );

  nspaceScenario(
    'Scenario: Clicking create in the intentions listing',
    async function (assert, nspace) {
      await visit('intentions', { dc: 'dc1' }, { nspace });

      await pages.intentions.create();
      assert.equal(currentURL(), nspaceURL(nspace, '/dc1/intentions/create'));

      await click('[data-test-breadcrumb-item]:first-child a');
      assert.equal(currentURL(), nspaceURL(nspace, '/dc1/intentions'));
    }
  );

  nspaceScenario(
    'Scenario: Clicking create in the tokens listing',
    async function (assert, nspace) {
      await visit('tokens', { dc: 'dc1' }, { nspace });

      await pages.tokens.create();
      assert.equal(currentURL(), nspaceURL(nspace, '/dc1/acls/tokens/create'));

      await click('[data-test-breadcrumb-item]:first-child a');
      assert.equal(currentURL(), nspaceURL(nspace, '/dc1/acls/tokens'));
    }
  );

  nspaceScenario(
    'Scenario: Clicking create in the policies listing',
    async function (assert, nspace) {
      await visit('policies', { dc: 'dc1' }, { nspace });

      await pages.policies.create();
      assert.equal(currentURL(), nspaceURL(nspace, '/dc1/acls/policies/create'));

      await click('[data-test-breadcrumb-item]:first-child a');
      assert.equal(currentURL(), nspaceURL(nspace, '/dc1/acls/policies'));
    }
  );
});
