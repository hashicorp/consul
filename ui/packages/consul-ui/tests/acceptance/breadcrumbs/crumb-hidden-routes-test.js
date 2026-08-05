/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

// Category D: Routes with breadcrumb: { show: false } must not render the
// breadcrumb list at all.
//
// Three classes of routes opt out:
//   1. dc.acls            — path: '/acls'           show: false
//   2. dc.services.instance — path: '/:name/instances/:node/:id'  show: false
//   3. Top-level pages    — index, settings, unavailable, notfound  show: false

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

  // ── dc.acls (show: false) ────────────────────────────────────────────────
  // The ACL index path /acls redirects, but the parent route config carries
  // show:false so no breadcrumb nav should appear on any ACL list page.

  nspaceScenario(
    'ACL tokens list page shows no breadcrumb list (dc.acls has show:false)',
    async function (assert) {
      api.server.createList('dc', 1, 'dc-1');
      api.server.createList('token', 1);

      await visit('tokens', { dc: 'dc-1' });

      assert
        .dom('[data-test-breadcrumb-list]')
        .doesNotExist('breadcrumb list is absent on the ACL tokens page');
    },
    { notNamespaceable: true }
  );

  nspaceScenario(
    'ACL policies list page shows no breadcrumb list',
    async function (assert) {
      api.server.createList('dc', 1, 'dc-1');
      api.server.createList('policy', 1);

      await visit('policies', { dc: 'dc-1' });

      assert
        .dom('[data-test-breadcrumb-list]')
        .doesNotExist('breadcrumb list is absent on the ACL policies page');
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
