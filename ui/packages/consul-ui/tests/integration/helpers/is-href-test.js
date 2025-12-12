/**
 * Copyright IBM Corp. 2014, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, skip } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { render } from '@ember/test-helpers';
import hbs from 'htmlbars-inline-precompile';

module('Integration | Helper | is-href', function (hooks) {
  setupRenderingTest(hooks);

  // Replace this with your real tests.
  skip('it renders', async function (assert) {
    this.set('inputValue', '1234');

    await render(hbs`{{is-href this.inputValue}}`);

    assert.equal(this.element.textContent.trim(), '1234');
  });
});
