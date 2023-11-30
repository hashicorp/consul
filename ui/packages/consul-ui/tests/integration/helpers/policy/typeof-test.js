/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { render } from '@ember/test-helpers';
import hbs from 'htmlbars-inline-precompile';

module('Integration | Helper | policy/typeof', function (hooks) {
  setupRenderingTest(hooks);

  test('it renders read-only cluster', async function (assert) {
    this.set('inputValue', {
      ID: '00000000-0000-0000-0000-000000000002',
      template: 'some-template',
    });

    await render(hbs`{{policy/typeof inputValue}}`);

    assert.equal(this.element.textContent.trim(), 'read-only');
  });
});
