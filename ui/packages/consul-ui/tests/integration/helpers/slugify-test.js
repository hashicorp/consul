/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import { module, skip, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { render } from '@ember/test-helpers';
import hbs from 'htmlbars-inline-precompile';

module('Integration | Helper | slugify', function (hooks) {
  setupRenderingTest(hooks);

  // Replace this with your real tests.
  test('it renders', async function (assert) {
    this.set('inputValue', 'Hi There');

    await render(hbs`{{slugify inputValue}}`);

    assert.equal(this.element.textContent.trim(), 'hi-there');
  });
  skip("it copes with more values such as ' etc");
});
