import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { render } from '@ember/test-helpers';
import hbs from 'htmlbars-inline-precompile';

module('helper:format-number', function(hooks) {
  setupRenderingTest(hooks);

  test('it renders a formatted number when passed a number', async function(assert) {
    this.set('inputValue', 1234);

    await render(hbs`{{format-number inputValue}}`);

    assert.dom('*').hasText('1,234');
  });
});
