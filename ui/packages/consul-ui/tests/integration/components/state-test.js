/**
 * Copyright IBM Corp. 2014, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { render } from '@ember/test-helpers';
import { hbs } from 'ember-cli-htmlbars';

module('Integration | Component | state', function (hooks) {
  setupRenderingTest(hooks);

  test('it renders', async function (assert) {
    // Set any properties with this.set('myProperty', 'value');
    // Handle any actions with this.set('myAction', function(val) { ... });

    this.set('state', {
      matches: function (id) {
        return id === 'idle';
      },
    });
    await render(hbs`
      <State @state={{this.state}} @matches="idle">
        Currently Idle
      </State>
    `);

    assert.strictEqual(this.element.textContent.trim(), 'Currently Idle');
    await render(hbs`
      <State @state={{this.state}} @matches="loading">
        Currently Idle
      </State>
    `);

    assert.strictEqual(this.element.textContent.trim(), '');
  });
});
