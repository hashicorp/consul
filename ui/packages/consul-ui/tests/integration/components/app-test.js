/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { render } from '@ember/test-helpers';
import hbs from 'htmlbars-inline-precompile';

module('Integration | Component | app', function (hooks) {
  setupRenderingTest(hooks);

  test('it renders the app shell containers in the expected order', async function (assert) {
    await render(hbs`
      <App>
        <:sideNav>
          <div data-test-side-nav>Side nav</div>
        </:sideNav>

        <:notifications as |app|>
          <app.Notification>Toast</app.Notification>
        </:notifications>

        <:main>
          <div data-test-main-content>Page body</div>
        </:main>
      </App>
    `);

    assert.dom('.app > .app-shell').exists();
    assert.dom('.app-shell > .hds-app-frame__modals .modal-layer').exists();
    assert.dom('.app-shell > .hds-app-frame__modals + main#hds-main.app-shell__main').exists();
    assert.dom('main#hds-main.app-shell__main').exists();
    assert.dom('.app-shell > .hds-app-frame__header').doesNotExist();
    assert.dom('.app-shell > .hds-app-frame__footer').doesNotExist();
  });

  test('it renders side nav, notifications, and page content in their shell regions', async function (assert) {
    await render(hbs`
      <App>
        <:sideNav>
          <div data-test-side-nav>Side nav</div>
        </:sideNav>

        <:notifications as |app|>
          <app.Notification>Toast</app.Notification>
        </:notifications>

        <:main>
          <div data-test-main-content>Page body</div>
        </:main>
      </App>
    `);

    assert.dom('.app-shell__sidebar [data-test-side-nav]').hasText('Side nav');
    assert.dom('.app-shell__main > .notifications .app-notification').hasText('Toast');
    assert.dom('main#hds-main.app-shell__main [data-test-main-content]').hasText('Page body');
  });
});
