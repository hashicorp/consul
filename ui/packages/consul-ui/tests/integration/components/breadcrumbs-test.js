/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { render, click } from '@ember/test-helpers';
import hbs from 'htmlbars-inline-precompile';

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

/** Build a minimal BreadcrumbItem array for testing. */
function makeItems(labels) {
  return labels.map((label, i) => ({
    label,
    route: `dc.${label.toLowerCase()}`,
    params: { dc: 'dc-1' },
    isCurrent: i === labels.length - 1,
    isClickable: i !== labels.length - 1,
  }));
}

module('Integration | Component | breadcrumbs', function (hooks) {
  setupRenderingTest(hooks);

  // ─── Rendering ────────────────────────────────────────────────────────────

  test('renders correct number of <li> items for a 3-item array', async function (assert) {
    this.set('items', makeItems(['Services', 'web', 'Instances']));

    await render(hbs`<Breadcrumbs @items={{this.items}} />`);

    assert.dom('[data-test-breadcrumb-item]').exists({ count: 3 });
  });

  test('renders nothing when @items is empty', async function (assert) {
    this.set('items', []);

    await render(hbs`<Breadcrumbs @items={{this.items}} />`);

    assert.dom('[data-test-breadcrumb-list]').doesNotExist();
  });

  test('renders a single item without error', async function (assert) {
    this.set('items', makeItems(['Services']));

    await render(hbs`<Breadcrumbs @items={{this.items}} />`);

    assert.dom('[data-test-breadcrumb-item]').exists({ count: 1 });
    assert.dom('[data-test-breadcrumb-current]').exists({ count: 1 });
  });

  // ─── Links vs current item ─────────────────────────────────────────────────

  test('last item is rendered as non-interactive current item (not a link)', async function (assert) {
    this.set('items', makeItems(['Services', 'web', 'Instances']));

    await render(hbs`<Breadcrumbs @items={{this.items}} />`);

    // HDS renders current item as a <div class="hds-breadcrumb__current">, not a link
    assert
      .dom('[data-test-breadcrumb-current] .hds-breadcrumb__current')
      .exists('current item renders as non-interactive div');
    assert
      .dom('[data-test-breadcrumb-current] .hds-breadcrumb__link')
      .doesNotExist('current item has no link');

    // Only 2 of the 3 items should be ancestor links
    const nonCurrent = this.element.querySelectorAll(
      '[data-test-breadcrumb-item]:not([data-test-breadcrumb-current])'
    );
    assert.strictEqual(nonCurrent.length, 2, '2 ancestor items present');
  });

  test('single-item list renders the only item as current', async function (assert) {
    this.set('items', makeItems(['Services']));

    await render(hbs`<Breadcrumbs @items={{this.items}} />`);

    assert.dom('[data-test-breadcrumb-current]').exists('single item is marked current');
    assert.dom('[data-test-breadcrumb-current] .hds-breadcrumb__current').exists();
  });

  // ─── Ancestor links ────────────────────────────────────────────────────────

  test('ancestor items render as <a> links with an href', async function (assert) {
    this.set('items', makeItems(['Services', 'web', 'Instances']));

    await render(hbs`<Breadcrumbs @items={{this.items}} />`);

    // The first two items are ancestors — HDS renders them via HdsInteractive
    // which produces an <a> element when @route is supplied.
    const ancestorItems = this.element.querySelectorAll(
      '[data-test-breadcrumb-item]:not([data-test-breadcrumb-current])'
    );
    ancestorItems.forEach((li, i) => {
      const link = li.querySelector('a');
      assert.ok(link, `item[${i}] contains an <a> element`);
      assert.ok(link.getAttribute('href'), `item[${i}] <a> has a non-empty href`);
    });
  });

  test('clicking an ancestor link does not throw', async function (assert) {
    // We cannot assert a full route transition in a rendering test (no live
    // router), but we can confirm the element is clickable without raising an
    // error — meaning the link is properly rendered and interactive.
    this.set('items', makeItems(['Services', 'web', 'Instances']));

    await render(hbs`<Breadcrumbs @items={{this.items}} />`);

    const firstAnchor = this.element.querySelector(
      '[data-test-breadcrumb-item]:not([data-test-breadcrumb-current]) a'
    );
    assert.ok(firstAnchor, 'ancestor <a> element found');

    let threw = false;
    try {
      await click(firstAnchor);
    } catch (_) {
      threw = true;
    }
    assert.false(threw, 'clicking ancestor link does not throw');
  });
});
