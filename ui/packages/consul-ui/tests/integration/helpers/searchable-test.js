/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, skip } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { render } from '@ember/test-helpers';
import { hbs } from 'ember-cli-htmlbars';

module('Integration | Helper | searchable', function (hooks) {
  setupRenderingTest(hooks);

  // Replace this with your real tests.
  skip('it renders', async function (assert) {
    this.set('inputValue', '1234');

    await render(hbs`{{searchable inputValue}}`);

    assert.equal(this.element.textContent.trim(), '1234');
  });
});
