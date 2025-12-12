/**
 * Copyright IBM Corp. 2014, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { render } from '@ember/test-helpers';
import hbs from 'htmlbars-inline-precompile';

module('Integration | Helper | service/external-source', function (hooks) {
  setupRenderingTest(hooks);

  // Replace this with your real tests.
  test('it renders', async function (assert) {
    this.set('inputValue', { Meta: { 'external-source': 'consul' } });

    await render(hbs`{{service/external-source this.inputValue}}`);

    assert.strictEqual(this.element.textContent.trim(), 'consul');
  });
  test('it renders prefixed', async function (assert) {
    this.set('inputValue', { Meta: { 'external-source': 'consul' } });

    await render(hbs`{{service/external-source this.inputValue prefix='external-source-'}}`);

    assert.strictEqual(this.element.textContent.trim(), 'external-source-consul');
  });
});
