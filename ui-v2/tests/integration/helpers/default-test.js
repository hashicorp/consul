import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { render } from '@ember/test-helpers';
import hbs from 'htmlbars-inline-precompile';

module('helper:default', function(hooks) {
  setupRenderingTest(hooks);

  // Replace this with your real tests.
  test('it renders', async function(assert) {
    this.set('inputValue', '1234');

    await render(hbs`{{default inputValue}}`);

    assert.dom('*').hasText('1234');
  });
  test('it renders the default value', async function(assert) {
    this.set('inputValue', '');

    await render(hbs`{{default inputValue '1234'}}`);

    assert.dom('*').hasText('1234');
  });
});
