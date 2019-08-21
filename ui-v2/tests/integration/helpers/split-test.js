import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { render } from '@ember/test-helpers';
import hbs from 'htmlbars-inline-precompile';

module('helper:split', function(hooks) {
  setupRenderingTest(hooks);

  // Replace this with your real tests.
  test('it renders', async function(assert) {
    this.set('inputValue', 'a,string,split,by,a,comma');

    await render(hbs`{{split inputValue}}`);

    assert.dom('*').hasText('a,string,split,by,a,comma');
  });
});
