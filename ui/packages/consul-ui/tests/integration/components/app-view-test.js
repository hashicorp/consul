/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { render } from '@ember/test-helpers';
import hbs from 'htmlbars-inline-precompile';

// ---------------------------------------------------------------------------
// Stub breadcrumb items used across tests
// ---------------------------------------------------------------------------
const STUB_ITEMS = [
  {
    label: 'Services',
    route: 'dc.services',
    params: { dc: 'dc-1' },
    isCurrent: false,
    isClickable: true,
  },
  {
    label: 'web',
    route: 'dc.services.show',
    params: { dc: 'dc-1' },
    isCurrent: true,
    isClickable: false,
  },
];

module('Integration | Component | app view', function (hooks) {
  setupRenderingTest(hooks);

  // ─── Baseline ─────────────────────────────────────────────────────────────

  test('it renders without breadcrumbs when nothing is provided', async function (assert) {
    await render(hbs`<AppView />`);
    assert.dom('.app-view').exists();
    assert.dom('[data-test-breadcrumbs]').exists();
    assert.dom('[data-test-breadcrumb-item]').doesNotExist();
  });

  // ─── Explicit <:breadcrumbs> block takes precedence ────────────────────────

  test('explicit <:breadcrumbs> block is rendered when provided', async function (assert) {
    await render(hbs`
      <AppView>
        <:breadcrumbs>
          <span data-test-manual-crumb>Manual crumb</span>
        </:breadcrumbs>
      </AppView>
    `);

    assert.dom('[data-test-manual-crumb]').exists('explicit breadcrumbs block is rendered');
    assert
      .dom('[data-test-breadcrumb-list]')
      .doesNotExist('service Breadcrumbs not rendered alongside manual block');
  });

  // ─── Service-driven fallback ──────────────────────────────────────────────

  test('service-computed breadcrumbs render when no <:breadcrumbs> block is given', async function (assert) {
    // Stub the breadcrumbs service to return a known set of items.
    const breadcrumbsService = this.owner.lookup('service:breadcrumbs');
    breadcrumbsService.shouldShowBreadcrumbs = () => true;
    breadcrumbsService.computeBreadcrumbs = () => STUB_ITEMS;

    // Stub the routlet service so paramsFor doesn't require a live router.
    const routletService = this.owner.lookup('service:routlet');
    routletService.paramsFor = () => ({ dc: 'dc-1' });

    // Stub the router service so currentRouteName is predictable.
    const routerService = this.owner.lookup('service:router');
    routerService.currentRouteName = 'dc.services.show';

    await render(hbs`<AppView />`);

    assert.dom('[data-test-breadcrumb-item]').exists({ count: 2 }, 'service items rendered');
  });

  // ─── @showBreadcrumb=false suppresses breadcrumbs ─────────────────────────

  test('@showBreadcrumb=false suppresses the breadcrumb nav content', async function (assert) {
    // Even if service would produce items, passing @showBreadcrumb=false must hide them.
    const breadcrumbsService = this.owner.lookup('service:breadcrumbs');
    breadcrumbsService.shouldShowBreadcrumbs = () => true;
    breadcrumbsService.computeBreadcrumbs = () => STUB_ITEMS;

    const routerService = this.owner.lookup('service:router');
    routerService.currentRouteName = 'dc.services.show';

    await render(hbs`<AppView @showBreadcrumb={{false}} />`);

    assert
      .dom('[data-test-breadcrumb-item]')
      .doesNotExist('breadcrumb items hidden when @showBreadcrumb=false');
    assert.dom('[data-test-breadcrumbs]').exists('nav wrapper still present');
  });

  // ─── shouldShowBreadcrumbs=false suppresses fallback ──────────────────────

  test('service breadcrumbs are not rendered when shouldShowBreadcrumbs returns false', async function (assert) {
    const breadcrumbsService = this.owner.lookup('service:breadcrumbs');
    breadcrumbsService.shouldShowBreadcrumbs = () => false;

    const routerService = this.owner.lookup('service:router');
    routerService.currentRouteName = 'dc.hidden';

    await render(hbs`<AppView />`);

    assert.dom('[data-test-breadcrumb-item]').doesNotExist();
  });
});
