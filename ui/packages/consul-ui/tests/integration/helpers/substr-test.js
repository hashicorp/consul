/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { render } from '@ember/test-helpers';
import hbs from 'htmlbars-inline-precompile';

module('Integration | Helper | substr', function (hooks) {
  setupRenderingTest(hooks);

  // Replace this with your real tests.
  test('it returns last 2 characters of string', async function (assert) {
    this.set('inputValue', 'd9a54409-648b-4327-974f-62a45c8c65f1');

    await render(hbs`{{substr inputValue -4}}`);

    assert.equal(this.element.textContent.trim(), '65f1');
  });
});
