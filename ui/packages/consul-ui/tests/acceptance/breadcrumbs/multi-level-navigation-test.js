/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

// Category E: Multi-level intermediate ancestor click.
//
// All existing navigation tests only click [data-test-breadcrumb-item]:first-child
// which always targets the root (list) ancestor.  These tests click an
// *intermediate* crumb in a 3-or-4-level chain to verify that each ancestor
// href is correct and navigates to the right page.
//
// Chains exercised:
//   Services → <service> → Instances → Health Checks   (4 levels)
//     • click 2nd crumb (<service>) → /dc-1/services/service-0/...
//     • click 1st crumb (Services)  → /dc-1/services
//
//   Auth Methods → <id> → Auth Method   (3 levels)
//     • click 2nd crumb (<id>)     → /dc-1/acls/auth-methods/<id>
//     • click 1st crumb (Auth Methods) → /dc-1/acls/auth-methods
//
//   Peers → <peer> → Imported Services  (3 levels)
//     • click 2nd crumb (<peer>)   → /dc-1/peers/peer-0
//     • click 1st crumb (Peers)    → /dc-1/peers

import { module } from 'qunit';
import { click } from '@ember/test-helpers';

import {
  setupAcceptanceTest,
  nspaceScenario,
  api,
  visit,
  page,
  currentURL,
  nspaceURL,
} from 'consul-ui/tests/helpers/acceptance';

module('Acceptance | breadcrumbs / multi-level-navigation', function (hooks) {
  setupAcceptanceTest(hooks);

  // ── 4-level chain: Services → <name> → Instances → Health Checks ────────

  nspaceScenario(
    'Scenario: Instance health-checks → click service crumb → lands on service detail',
    async function (assert, nspace) {
      api.server.createList('dc', 1, 'dc-1');
      api.server.createList('service', 1);
      api.server.createList('instance', 2);

      await visit('service', { dc: 'dc-1', service: 'service-0' }, { nspace });
      await page().tabs.instances();
      await page().instances.objectAt(0).instance();

      // Confirm we are on the Health Checks sub-page (4-deep crumb chain).
      assert.ok(
        currentURL().includes('/dc-1/services/service-0/instances/'),
        `Expected instance detail URL, got ${currentURL()}`
      );
      assert.dom('[data-test-breadcrumb-item]').exists({ count: 4 }, '4 crumbs rendered');

      // Click the 2nd crumb (service name) → should land on service show.
      await click('[data-test-breadcrumb-item]:nth-child(2) a');
      assert.ok(
        currentURL().match(/\/dc-1\/services\/service-0\//),
        `Expected service detail URL, got ${currentURL()}`
      );
    },
    { notNamespaceable: true }
  );

  nspaceScenario(
    'Scenario: Instance health-checks → click Services crumb → lands on services list',
    async function (assert, nspace) {
      api.server.createList('dc', 1, 'dc-1');
      api.server.createList('service', 1);
      api.server.createList('instance', 2);

      await visit('service', { dc: 'dc-1', service: 'service-0' }, { nspace });
      await page().tabs.instances();
      await page().instances.objectAt(0).instance();

      // Click the 1st crumb (Services list).
      await click('[data-test-breadcrumb-item]:first-child a');
      assert.ok(
        currentURL().includes(nspaceURL(nspace, '/dc-1/services')),
        `Expected services list URL, got ${currentURL()}`
      );
    },
    { notNamespaceable: true }
  );

  // ── 3-level chain: Auth Methods → <id> → Auth Method ────────────────────
  // dc.acls.auth-methods.show.index always redirects to the 'auth-method'
  // child route (see app/routes/dc/acls/auth-methods/show/index.js), so
  // clicking into a row from the list always lands on the 'auth-method'
  // sub-tab.  The crumb chain is always 3 levels deep:
  //   Auth Methods → <id> → Auth Method  (current)

  nspaceScenario(
    'Scenario: Auth-method sub-tab → click auth-method detail crumb → lands on auth-method detail',
    async function (assert, nspace) {
      api.server.createList('dc', 1, 'dc-1');
      api.server.createList('authMethod', 3);

      await visit('authMethods', { dc: 'dc-1' }, { nspace });
      await page().authMethods.objectAt(0).authMethod();

      // The show/index route immediately redirects to /auth-method, so we
      // always land on the 3-crumb chain — no conditional needed.
      assert.dom('[data-test-breadcrumb-item]').exists({ count: 3 }, '3 crumbs rendered');

      // Click the 2nd crumb (<id>) → the show/index redirect fires again and
      // settles on /<id>/auth-method, so the URL must match that exact shape.
      await click('[data-test-breadcrumb-item]:nth-child(2) a');
      assert.ok(
        currentURL().match(/\/dc-1\/acls\/auth-methods\/[^/]+\/auth-method$/),
        `Expected URL matching /dc-1/acls/auth-methods/<id>/auth-method, got ${currentURL()}`
      );
    },
    { notNamespaceable: true }
  );

  nspaceScenario(
    'Scenario: Auth-method detail → click Auth Methods crumb → lands on auth-methods list',
    async function (assert, nspace) {
      api.server.createList('dc', 1, 'dc-1');
      api.server.createList('authMethod', 3);

      await visit('authMethods', { dc: 'dc-1' }, { nspace });
      await page().authMethods.objectAt(0).authMethod();

      // The first crumb should always be the Auth Methods list page.
      await click('[data-test-breadcrumb-item]:first-child a');
      assert.ok(
        currentURL().includes('/dc-1/acls/auth-methods'),
        `Expected auth-methods list URL, got ${currentURL()}`
      );
      assert.notOk(
        currentURL().includes('/dc-1/acls/auth-methods/'),
        'URL is the list page, not a detail page'
      );
    },
    { notNamespaceable: true }
  );

  // ── 3-level chain: Peers → <name> → Imported Services ───────────────────
  // dc.peers.show.index always redirects to dc.peers.show.imported
  // (see app/controllers/dc/peers/show/index.js → transitionToImported).
  // visit('peer', ...) therefore always lands on /imported-services and the
  // crumb chain is always 3 levels deep:
  //   Peers → <name> → Imported Services  (current)

  nspaceScenario(
    'Scenario: Peer imported-services tab → click peer name crumb → lands on peer detail',
    async function (assert, nspace) {
      api.server.createList('dc', 1, 'dc-1');
      api.server.createList('peer', 1);

      await visit('peer', { dc: 'dc-1', peer: 'peer-0' }, { nspace });

      // The show/index redirect always fires, so we always land on
      // /imported-services — no conditional needed.
      assert.dom('[data-test-breadcrumb-item]').exists({ count: 3 }, '3 crumbs rendered');

      // Click the 2nd crumb (<name>) → the show/index redirect fires again
      // and settles on /<name>/imported-services.
      await click('[data-test-breadcrumb-item]:nth-child(2) a');
      assert.ok(
        currentURL().match(/\/dc-1\/peers\/[^/]+\/imported-services$/),
        `Expected URL matching /dc-1/peers/<name>/imported-services, got ${currentURL()}`
      );
    },
    { notNamespaceable: true }
  );

  nspaceScenario(
    'Scenario: Peer detail → click Peers crumb → lands on peers list',
    async function (assert, nspace) {
      api.server.createList('dc', 1, 'dc-1');
      api.server.createList('peer', 1);

      await visit('peer', { dc: 'dc-1', peer: 'peer-0' }, { nspace });

      await click('[data-test-breadcrumb-item]:first-child a');
      assert.ok(
        currentURL().includes('/dc-1/peers'),
        `Expected peers list URL, got ${currentURL()}`
      );
      assert.notOk(
        currentURL().includes('/dc-1/peers/peer-0'),
        'URL is the list page, not the peer detail page'
      );
    },
    { notNamespaceable: true }
  );
});
