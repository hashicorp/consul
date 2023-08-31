/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { render } from '@ember/test-helpers';
import { hbs } from 'ember-cli-htmlbars';

module('Integration | Component | auth-profile', function (hooks) {
  setupRenderingTest(hooks);

  test('it renders', async function (assert) {
    // Set any properties with this.set('myProperty', 'value');
    // Handle any actions with this.set('myAction', function(val) { ... });

    await render(hbs`<AuthProfile />`);

    assert.notStrictEqual(this.element.textContent.indexOf('AccessorID'), -1);

    // Template block usage:
    await render(hbs`
      <AuthProfile></AuthProfile>
    `);

    assert.notStrictEqual(this.element.textContent.indexOf('AccessorID'), -1);
  });
});
