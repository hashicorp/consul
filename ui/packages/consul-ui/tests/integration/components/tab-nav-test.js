/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { click, render } from '@ember/test-helpers';
import hbs from 'htmlbars-inline-precompile';

module('Integration | Component | tab nav', function (hooks) {
  setupRenderingTest(hooks);

  test('it renders HDS tabs and transitions to route-backed items', async function (assert) {
    const router = this.owner.lookup('service:router');
    router.transitionTo = (route) => {
      assert.strictEqual(route, 'second.route', 'transitions to the selected route');
      this.set('items', [
        {
          label: 'First tab',
          route: 'first.route',
          selected: false,
        },
        {
          label: 'Second tab',
          route: 'second.route',
          selected: true,
        },
      ]);
    };
    this.set('items', [
      {
        label: 'First tab',
        route: 'first.route',
        selected: true,
      },
      {
        label: 'Second tab',
        route: 'second.route',
        selected: false,
      },
    ]);
    await render(hbs`
      <TabNav @items={{this.items}} />
    `);

    assert.dom('.hds-tabs').exists('renders the HDS Tabs component');
    assert
      .dom('[data-test-tab="tab_first-tab"] button')
      .hasAttribute('aria-selected', 'true', 'marks the selected tab');
    assert
      .dom('[data-test-tab="tab_second-tab"] button')
      .hasAttribute('aria-selected', 'false', 'marks the unselected tab');

    await click('[data-test-tab="tab_second-tab"] button');

    assert
      .dom('[data-test-tab="tab_second-tab"]')
      .hasClass('hds-tabs__tab--is-selected', 'HDS updates the selected tab');
  });

  test('it preserves state-backed tab callbacks', async function (assert) {
    this.set('items', [
      { label: 'Token', selected: true },
      { label: 'SSO', selected: false },
    ]);
    this.set('onClick', (label) => {
      assert.strictEqual(label, 'SSO', 'passes the uppercased label to @onclick');
    });
    this.set('onTabClicked', (item) => {
      assert.strictEqual(item.label, 'SSO', 'passes the item to @onTabClicked');
    });

    await render(hbs`
      <TabNav
        @items={{this.items}}
        @onclick={{this.onClick}}
        @onTabClicked={{this.onTabClicked}}
      />
    `);

    await click('[data-test-tab="tab_sso"] button');
  });

  test('it routes conditional tabs by their visible order', async function (assert) {
    const router = this.owner.lookup('service:router');
    router.transitionTo = (route) => {
      assert.strictEqual(route, 'exposed.route', 'transitions to the visible conditional tab');
      this.set('items', [
        { label: 'Health checks', route: 'health.route', selected: false },
        { label: 'Upstreams', route: 'upstreams.route', selected: false },
        { label: 'Exposed paths', route: 'exposed.route', selected: true },
        { label: 'Metadata', route: 'metadata.route', selected: false },
      ]);
    };
    this.set('items', [
      { label: 'Health checks', route: 'health.route', selected: true },
      { label: 'Metadata', route: 'metadata.route', selected: false },
    ]);

    await render(hbs`
      <TabNav @items={{this.items}} />
    `);

    this.set('items', [
      { label: 'Health checks', route: 'health.route', selected: true },
      { label: 'Upstreams', route: 'upstreams.route', selected: false },
      { label: 'Exposed paths', route: 'exposed.route', selected: false },
      { label: 'Metadata', route: 'metadata.route', selected: false },
    ]);

    await click('[data-test-tab="tab_exposed-paths"] button');

    assert
      .dom('[data-test-tab="tab_exposed-paths"] button')
      .hasAttribute('aria-selected', 'true', 'marks the conditional tab as selected');
  });
});
