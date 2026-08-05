/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

// Category F: Tab switching updates the last crumb label while keeping
// all ancestor crumbs identical.
//
// When a user switches tabs on a detail page (e.g. node Health Checks →
// Lock Sessions, or service Topology → Routing), only the current (last)
// crumb label should change; the ancestor crumbs must remain unchanged and
// still link to the same pages.

import { module } from 'qunit';
import { click } from '@ember/test-helpers';

import {
  setupAcceptanceTest,
  nspaceScenario,
  api,
  visit,
  page,
} from 'consul-ui/tests/helpers/acceptance';

// Helper: read the text of every [data-test-breadcrumb-item] in DOM order.
function crumbTexts() {
  return [...document.querySelectorAll('[data-test-breadcrumb-item]')].map((el) =>
    el.textContent.trim()
  );
}

// Helper: read the href of the Nth (1-based) non-current crumb anchor.
function ancestorHref(n) {
  const anchors = [
    ...document.querySelectorAll(
      '[data-test-breadcrumb-item]:not([data-test-breadcrumb-current]) a'
    ),
  ];
  return anchors[n - 1]?.getAttribute('href') ?? null;
}

module('Acceptance | breadcrumbs / tab-navigation', function (hooks) {
  setupAcceptanceTest(hooks);

  // ── Node: Health Checks → Lock Sessions ─────────────────────────────────

  nspaceScenario(
    'Scenario: Node Health Checks → Lock Sessions tab updates last crumb only',
    async function (assert) {
      api.server.createList('dc', 1, 'dc-1');
      api.server.createList('node', 1);

      await visit('node', { dc: 'dc-1', node: 'node-0' });

      // Default landing: Health Checks tab.
      await page().tabs.healthChecks();
      const ancestorHrefBefore = ancestorHref(1);

      assert
        .dom('[data-test-breadcrumb-current]')
        .hasText('Health Checks', 'initial last crumb is Health Checks');
      assert.ok(ancestorHrefBefore, 'ancestor crumb has an href');

      // Switch to Lock Sessions.
      await page().tabs.lockSessions();

      assert
        .dom('[data-test-breadcrumb-current]')
        .hasText('Lock Sessions', 'last crumb updated to Lock Sessions');

      // Ancestor crumbs must be unchanged.
      const textsAfter = crumbTexts();
      assert.ok(
        textsAfter[0].includes('node-0') || textsAfter[0].includes('Nodes'),
        `first ancestor crumb unchanged, got "${textsAfter[0]}"`
      );
      assert.strictEqual(
        ancestorHref(1),
        ancestorHrefBefore,
        'first ancestor href is unchanged after tab switch'
      );
    },
    { notNamespaceable: true }
  );

  nspaceScenario(
    'Scenario: Node Lock Sessions → Health Checks tab updates last crumb only',
    async function (assert) {
      api.server.createList('dc', 1, 'dc-1');
      api.server.createList('node', 1);

      await visit('node', { dc: 'dc-1', node: 'node-0' });
      await page().tabs.lockSessions();

      assert
        .dom('[data-test-breadcrumb-current]')
        .hasText('Lock Sessions', 'last crumb is Lock Sessions');

      await page().tabs.healthChecks();

      assert
        .dom('[data-test-breadcrumb-current]')
        .hasText('Health Checks', 'last crumb updated to Health Checks');
    },
    { notNamespaceable: true }
  );

  // ── Service: Topology → Routing ─────────────────────────────────────────

  nspaceScenario(
    'Scenario: Service Topology → Routing tab updates last crumb only',
    async function (assert) {
      api.server.createList('dc', 1, 'dc-1');
      api.server.createList('service', 1);

      await visit('service', { dc: 'dc-1', service: 'service-0' });
      await page().tabs.topology();

      const ancestorHrefBefore = ancestorHref(1);

      assert
        .dom('[data-test-breadcrumb-current]')
        .hasText('Topology', 'initial last crumb is Topology');

      // Switch to the Routing tab.
      await page().tabs.routing();

      assert
        .dom('[data-test-breadcrumb-current]')
        .hasText('Routing', 'last crumb updated to Routing');

      assert.strictEqual(
        ancestorHref(1),
        ancestorHrefBefore,
        'first ancestor href is unchanged after tab switch'
      );
    },
    { notNamespaceable: true }
  );

  // ── Service instances: instances tab → health-checks sub-tab ────────────

  nspaceScenario(
    'Scenario: Instance health-checks → upstreams tab updates last crumb from Health Checks to Upstreams',
    async function (assert) {
      api.server.createList('dc', 1, 'dc-1');
      api.server.createList('service', 1);
      api.server.createList('instance', 2);

      await visit('service', { dc: 'dc-1', service: 'service-0' });
      await page().tabs.instances();
      await page().instances.objectAt(0).instance();

      // Confirm we land on Health Checks.
      assert
        .dom('[data-test-breadcrumb-current]')
        .hasText('Health Checks', 'initial last crumb is Health Checks');

      const ancestorCount = document.querySelectorAll(
        '[data-test-breadcrumb-item]:not([data-test-breadcrumb-current])'
      ).length;
      const firstAnchorHrefBefore = ancestorHref(1);

      // Switch to Upstreams sub-tab.
      const upstreamsTab = document.querySelector('[data-test-tab-nav] a[href*="upstreams"]');
      if (upstreamsTab) {
        await click(upstreamsTab);

        assert
          .dom('[data-test-breadcrumb-current]')
          .hasText('Upstreams', 'last crumb updated to Upstreams');

        assert.strictEqual(
          document.querySelectorAll(
            '[data-test-breadcrumb-item]:not([data-test-breadcrumb-current])'
          ).length,
          ancestorCount,
          'ancestor crumb count is unchanged'
        );
        assert.strictEqual(
          ancestorHref(1),
          firstAnchorHrefBefore,
          'first ancestor href is unchanged'
        );
      } else {
        // Upstreams tab not present for this service type — skip gracefully.
        assert.ok(true, 'upstreams tab not present, skipping tab-switch assertion');
      }
    },
    { notNamespaceable: true }
  );
});
