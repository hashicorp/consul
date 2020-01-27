import { module, skip, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { render } from '@ember/test-helpers';
import hbs from 'htmlbars-inline-precompile';

module('helper:slugify', function(hooks) {
  setupRenderingTest(hooks);

  // Replace this with your real tests.
  test('it renders', async function(assert) {
    this.set('inputValue', 'Hi There');

    await render(hbs`{{slugify inputValue}}`);

    assert.dom('*').hasText('hi-there');
  });
  skip("it copes with more values such as ' etc");
});
