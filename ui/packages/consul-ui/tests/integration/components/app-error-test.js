/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { render } from '@ember/test-helpers';
import hbs from 'htmlbars-inline-precompile';

module('Integration | Component | app-error', function (hooks) {
  setupRenderingTest(hooks);

  test('it renders a full error page by default', async function (assert) {
    this.set('error', { status: '500', message: 'Consul returned an error' });

    await render(hbs`<AppError @error={{this.error}} />`);

    assert.dom('.app-view').exists('renders its own AppView');
    assert.dom('h1').hasText('Error 500');
    assert.dom('h2').hasText('Consul returned an error');
  });

  test('it renders only the error state with @nested', async function (assert) {
    this.set('error', { status: '500', message: 'Consul returned an error' });

    await render(hbs`<AppError @nested={{true}} @error={{this.error}} />`);

    assert.dom('.app-view').doesNotExist('does not render a nested AppView');
    assert.dom('h1').doesNotExist('leaves the page heading to the surrounding page');
    assert.dom('h2').hasText('Consul returned an error');
    assert.dom('[data-test-status]').hasText('Error 500');
  });

  test('it shows the not authorized state for a 403 with @nested', async function (assert) {
    this.set('error', { status: '403' });

    await render(hbs`<AppError @nested={{true}} @error={{this.error}} />`);

    assert.dom('.app-view').doesNotExist();
    assert.dom('h2').hasText('You are not authorized');
  });
});
