/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

// Category H: Acceptance-only edge cases.
//
// H1 (_modelsFor includes dc) and H3 (routing-config chain) are pure service
// logic and live in tests/unit/services/breadcrumbs-test.js.
//
// The two cases below require a live router and cannot be tested in isolation:
//
//   H2 — Service name with slashes: URL encoding/decoding of the service name
//        param only happens via the real router's dynamic segment resolution.
//        The crumb label must show the decoded name; back-navigation must work.
//
//   H4 — DC switch: visiting a page in dc-2 after dc-1 must update the ancestor
//        crumb href to reference /dc-2/ — only verifiable via two real visits
//        and a live DOM href read.

import { module } from 'qunit';
import { click } from '@ember/test-helpers';

import {
  setupAcceptanceTest,
  nspaceScenario,
  api,
  visit,
  page,
  currentURL,
} from 'consul-ui/tests/helpers/acceptance';

// Return the href of the first ancestor (non-current) crumb link.
function firstAnchorHref() {
  return (
    document
      .querySelector('[data-test-breadcrumb-item]:not([data-test-breadcrumb-current]) a')
      ?.getAttribute('href') ?? null
  );
}

module('Acceptance | breadcrumbs / edge-cases', function (hooks) {
  setupAcceptanceTest(hooks);

  // ── H2: Service name with slashes ────────────────────────────────────────
  // The service name "hashicorp/service/service-0" is URL-encoded in the
  // address bar.  The crumb label must show the decoded form and
  // back-navigation must work without throwing.

  nspaceScenario(
    'Scenario: Service with slashes in its name renders crumb with decoded name',
    async function (assert, nspace) {
      api.server.createList('dc', 1, 'dc1');
      api.server.createList('node', 1);
      api.server.createList('service', 1, [{ Name: 'hashicorp/service/service-0' }]);

      await visit('services', { dc: 'dc1' }, { nspace });

      // Navigate into the service (the page object encodes the name).
      // This lands on dc.services.show.instances (the default child route),
      // so the breadcrumb trail is: Services → <service-name> → Instances.
      // [data-test-breadcrumb-current] therefore shows "Instances".
      await page().services.objectAt(0).service();

      // The service-name crumb is the last *non-current* (ancestor) crumb.
      // Query all non-current items and take the last one.
      const ancestorItems = document.querySelectorAll(
        '[data-test-breadcrumb-item]:not([data-test-breadcrumb-current])'
      );
      const serviceNameCrumb = ancestorItems[ancestorItems.length - 1];
      const serviceNameText = serviceNameCrumb?.textContent?.trim();

      assert.ok(
        serviceNameText && !serviceNameText.includes('%2F'),
        `service-name crumb is decoded, got "${serviceNameText}"`
      );
      assert.ok(
        (serviceNameText && serviceNameText.includes('hashicorp')) ||
          serviceNameText?.includes('service-0'),
        `service-name crumb contains part of the service name, got "${serviceNameText}"`
      );

      // The first ancestor crumb ("Services") must navigate back to the list.
      const ancestorLink = ancestorItems[0]?.querySelector('a');
      assert.ok(ancestorLink, 'ancestor crumb has a link');

      let threw = false;
      try {
        await click(ancestorLink);
      } catch (_) {
        threw = true;
      }
      assert.false(threw, 'clicking ancestor crumb of slash-name service does not throw');
      assert.ok(
        currentURL().includes('/dc1/services'),
        `back-navigation lands on services list, got ${currentURL()}`
      );
    },
    { notNamespaceable: true }
  );

  // ── H4: DC switch updates ancestor crumb href ────────────────────────────
  // After navigating to dc-2, the ancestor breadcrumb href must reference
  // /dc-2/ — not /dc-1/ left over from the previous visit.

  nspaceScenario(
    'Scenario: After DC switch the ancestor crumb href references the new datacenter',
    async function (assert, nspace) {
      api.server.createList('dc', 2, ['dc-1', 'dc-2']);
      api.server.createList('service', 1);

      // First visit dc-1.
      await visit('service', { dc: 'dc-1', service: 'service-0' }, { nspace });
      const hrefDc1 = firstAnchorHref();
      assert.ok(
        hrefDc1?.includes('/dc-1/'),
        `ancestor href on dc-1 contains /dc-1/, got "${hrefDc1}"`
      );

      // Visit the same page under dc-2.
      await visit('service', { dc: 'dc-2', service: 'service-0' }, { nspace });
      const hrefDc2 = firstAnchorHref();
      assert.ok(
        hrefDc2?.includes('/dc-2/'),
        `ancestor href on dc-2 contains /dc-2/, got "${hrefDc2}"`
      );
      assert.notOk(
        hrefDc2?.includes('/dc-1/'),
        `ancestor href on dc-2 must not contain /dc-1/, got "${hrefDc2}"`
      );
    },
    { notNamespaceable: true }
  );
});
