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
    assert.strictEqual(
      this.owner.lookup('service:navigation-focus').tabRoute,
      'second.route',
      'the route change is recorded as tab navigation so focus stays on the tab'
    );
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

  test('it attaches the tooltip directly to the focusable tab button', async function (assert) {
    this.set('items', [
      { label: 'Imported Services', selected: true, tooltip: 'Services imported from peer' },
      { label: 'Exported Services', selected: false },
    ]);

    await render(hbs`
      <TabNav @items={{this.items}} />
    `);

    const button = this.element.querySelector('[data-test-tab="tab_imported-services"] button');

    assert.ok(button._tippy, 'the tab button itself is the tippy reference/trigger element');
    assert.strictEqual(
      button._tippy.props.content,
      'Services imported from peer',
      'the tooltip content is the tab tooltip text'
    );
    assert
      .dom('[data-test-tab="tab_imported-services"] span[tabindex="-1"]')
      .doesNotExist(
        'no nested, keyboard-unreachable tooltip trigger is rendered inside the tab button'
      );
    assert
      .dom('[data-test-tab="tab_exported-services"] button')
      .exists('tabs without a tooltip still render normally');
  });

  test('it renders yielded content only inside the selected tabpanel', async function (assert) {
    this.set('items', [
      { label: 'First tab', selected: true },
      { label: 'Second tab', selected: false },
    ]);

    await render(hbs`
      <TabNav @items={{this.items}} as |tab|>
        <p data-test-content>{{tab.item.label}} {{tab.index}} {{tab.isVisible}}</p>
      </TabNav>
    `);

    const firstButton = this.element.querySelector('[data-test-tab="tab_first-tab"] button');
    assert
      .dom('[data-test-tab-panel="tab_first-tab"]')
      .hasAttribute('role', 'tabpanel')
      .hasAttribute('aria-labelledby', firstButton.id, 'the panel is labelled by its tab')
      .doesNotHaveAttribute('hidden');
    assert
      .dom('[data-test-tab-panel="tab_first-tab"] [data-test-content]')
      .hasText('First tab 0 true', 'the content is rendered inside the selected tabpanel');
    assert.dom('[data-test-content]').exists({ count: 1 }, 'the content is rendered only once');
    assert.dom('[data-test-tab-panel="tab_second-tab"]').hasAttribute('hidden');
    assert.dom('[data-test-tab-panel="tab_second-tab"]').hasNoText('the unselected panel is empty');

    this.set('items', [
      { label: 'First tab', selected: false },
      { label: 'Second tab', selected: true },
    ]);

    const secondButton = this.element.querySelector('[data-test-tab="tab_second-tab"] button');
    assert
      .dom('[data-test-tab-panel="tab_second-tab"]')
      .hasAttribute('aria-labelledby', secondButton.id)
      .doesNotHaveAttribute('hidden');
    assert
      .dom('[data-test-tab-panel="tab_second-tab"] [data-test-content]')
      .hasText('Second tab 1 true', 'the content moves into the newly selected tabpanel');
    assert.dom('[data-test-content]').exists({ count: 1 }, 'the content is still rendered once');
    assert.dom('[data-test-tab-panel="tab_first-tab"]').hasAttribute('hidden').hasNoText();
  });

  test('it keeps route content in the panel of the route the router is on', async function (assert) {
    const router = this.owner.lookup('service:router');
    // `is-href` marks the next tab selected as soon as a transition starts,
    // while the outlet still renders the current route until it completes.
    Object.defineProperty(router, 'currentRouteName', {
      configurable: true,
      get: () => 'dc.first.index',
    });
    this.set('items', [
      { label: 'First tab', route: 'dc.first', selected: false },
      { label: 'Second tab', route: 'dc.second', selected: true },
    ]);

    await render(hbs`
      <TabNav @items={{this.items}} as |tab|>
        <p data-test-content>{{tab.item.label}}</p>
      </TabNav>
    `);

    assert
      .dom('[data-test-tab="tab_second-tab"] button')
      .hasAttribute('aria-selected', 'true', 'the next tab is already selected');
    assert
      .dom('[data-test-tab-panel="tab_first-tab"] [data-test-content]')
      .hasText('First tab', 'the content stays in the current route panel until it changes');
    assert.dom('[data-test-content]').exists({ count: 1 });
  });

  test('it renders yielded content into every tabpanel with @renderAllPanels', async function (assert) {
    this.set('items', [
      { label: 'Token', selected: true },
      { label: 'SSO', selected: false },
    ]);

    await render(hbs`
      <TabNav @items={{this.items}} @renderAllPanels={{true}} as |tab|>
        <p data-test-content>{{tab.item.label}} {{tab.isVisible}}</p>
      </TabNav>
    `);

    assert
      .dom('[data-test-tab-panel="tab_token"] [data-test-content]')
      .hasText('Token true', 'the selected panel is told it is visible');
    assert
      .dom('[data-test-tab-panel="tab_sso"] [data-test-content]')
      .hasText('SSO false', 'the hidden panel is rendered and told it is not visible');
    assert.dom('[data-test-tab-panel="tab_sso"]').hasAttribute('hidden');
  });
});
