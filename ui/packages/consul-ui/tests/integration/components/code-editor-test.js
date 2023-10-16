/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { render } from '@ember/test-helpers';
import hbs from 'htmlbars-inline-precompile';

module('Integration | Component | code editor', function (hooks) {
  setupRenderingTest(hooks);

  test('it renders', async function (assert) {
    // Set any properties with this.set('myProperty', 'value');
    // Handle any actions with this.on('myAction', function(val) { ... });

    await render(hbs`{{code-editor}}`);

    // this test is just to prove it renders something without producing
    // an error. It renders the number 1, but seems to also render some sort of trailing space
    // so just check for presence of CodeMirror
    assert.equal(this.element.querySelectorAll('.CodeMirror').length, 1);

    // Template block usage:
    await render(hbs`
      {{#code-editor}}{{/code-editor}}
    `);
    assert.equal(this.element.querySelectorAll('.CodeMirror').length, 1);
  });
});
