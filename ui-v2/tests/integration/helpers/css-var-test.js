import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { render } from '@ember/test-helpers';
import hbs from 'htmlbars-inline-precompile';

module('helper:css-var', function(hooks) {
  setupRenderingTest(hooks);

  // Replace this with your real tests.
  test("it renders nothing if the variable doesn't exist", async function(assert) {
    this.set('inputValue', '1234');

    await render(hbs`{{css-var inputValue}}`);

    assert.dom('*').hasText('');
  });
  test("it renders a default if the variable doesn't exist", async function(assert) {
    this.set('inputValue', '1234');

    await render(hbs`{{css-var inputValue 'none'}}`);

    assert.dom('*').hasText('none');
  });
});
