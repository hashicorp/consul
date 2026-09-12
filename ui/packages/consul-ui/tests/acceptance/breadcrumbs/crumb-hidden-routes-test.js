/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

// Category D: Routes where breadcrumbs are hidden or show a minimal single crumb.
//
// dc.acls has breadcrumb: { show: false } — so no ACLs ancestor crumb appears.
// But its *children* (tokens, policies, …) each declare their own breadcrumb
// label with no parent, so those list pages render a single-item breadcrumb list
// (just "Tokens" / "Policies") rather than no list at all.
//
// Three classes of routes opt out entirely (no list rendered):
//   1. dc.services.instance — path: '/:name/instances/:node/:id'  show: false
//   2. Top-level pages    — index, settings, unavailable, notfound  show: false

import { module } from 'qunit';

import {
  setupAcceptanceTest,
  nspaceScenario,
  api,
  visit,
  page,
} from 'consul-ui/tests/helpers/acceptance';

module('Acceptance | breadcrumbs / hidden-routes', function (hooks) {
  setupAcceptanceTest(hooks);

  // ── dc.acls.tokens / dc.acls.policies (dc.acls parent has show:false) ───
  // dc.acls itself has breadcrumb: { show: false }, so it never appears as an
  // ancestor crumb. dc.acls.tokens and dc.acls.policies each have their own
  // label ('Tokens' / 'Policies') with no declared parent, so the breadcrumb
  // list renders with exactly one item — no ACLs crumb above it.

  nspaceScenario(
    'ACL tokens list page shows a single "Tokens" crumb with no ACLs ancestor',
    async function (assert) {
      api.server.createList('dc', 1, 'dc-1');
      api.server.createList('token', 1);

      await visit('tokens', { dc: 'dc-1' });

      assert
        .dom('[data-test-breadcrumb-list]')
        .exists('breadcrumb list is present on the ACL tokens page');

      const items = document.querySelectorAll('[data-test-breadcrumb-item]');
      assert.strictEqual(items.length, 1, 'exactly one breadcrumb item');
      assert.strictEqual(
        items[0]?.textContent?.trim(),
        'Tokens',
        'the single crumb label is "Tokens"'
      );
      assert
        .dom('[data-test-breadcrumb-item]:not([data-test-breadcrumb-current]) a')
        .doesNotExist('no ancestor link — Tokens crumb has no parent');
    },
    { notNamespaceable: true }
  );

  nspaceScenario(
    'ACL policies list page shows a single "Policies" crumb with no ACLs ancestor',
    async function (assert) {
      api.server.createList('dc', 1, 'dc-1');
      api.server.createList('policy', 1);

      await visit('policies', { dc: 'dc-1' });

      assert
        .dom('[data-test-breadcrumb-list]')
        .exists('breadcrumb list is present on the ACL policies page');

      const items = document.querySelectorAll('[data-test-breadcrumb-item]');
      assert.strictEqual(items.length, 1, 'exactly one breadcrumb item');
      assert.strictEqual(
        items[0]?.textContent?.trim(),
        'Policies',
        'the single crumb label is "Policies"'
      );
      assert
        .dom('[data-test-breadcrumb-item]:not([data-test-breadcrumb-current]) a')
        .doesNotExist('no ancestor link — Policies crumb has no parent');
    },
    { notNamespaceable: true }
  );

  // ── dc.services.instance (show: false) ──────────────────────────────────
  // The instance *redirect* route (/:name/instances/:node/:id) has show:false.
  // The redirect immediately takes the browser to the healthchecks sub-route
  // which does have a breadcrumb, so visiting the sub-route directly confirms
  // the correct route (healthchecks) is showing its own breadcrumb — while the
  // parent instance route itself (if somehow directly visited) would not.
  //
  // We verify the redirect target (healthchecks) DOES render breadcrumbs as a
  // sanity check that show:false on the parent doesn't bleed into child routes.

  nspaceScenario(
    'Service instance health-checks page renders breadcrumbs (child of show:false parent)',
    async function (assert) {
      api.server.createList('dc', 1, 'dc-1');
      api.server.createList('service', 1);
      api.server.createList('instance', 2);

      await visit('service', { dc: 'dc-1', service: 'service-0' });
      await page().tabs.instances();
      await page().instances.objectAt(0).instance();

      assert
        .dom('[data-test-breadcrumb-list]')
        .exists('healthchecks sub-route renders breadcrumbs even though parent has show:false');
    },
    { notNamespaceable: true }
  );

  // ── Settings (show: false) ───────────────────────────────────────────────

  nspaceScenario(
    'Settings page shows no breadcrumb list',
    async function (assert) {
      await visit('settings', {});

      assert
        .dom('[data-test-breadcrumb-list]')
        .doesNotExist('breadcrumb list is absent on the settings page');
    },
    { notNamespaceable: true }
  );
});
