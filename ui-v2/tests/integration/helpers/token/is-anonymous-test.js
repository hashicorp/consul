import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { render } from '@ember/test-helpers';
import hbs from 'htmlbars-inline-precompile';

module('helper:token/is-anonymous', function(hooks) {
  setupRenderingTest(hooks);

  // Replace this with your real tests.
  test('it renders', async function(assert) {
    this.set('inputValue', { AccessorID: '00000' });

    await render(hbs`{{token/is-anonymous inputValue}}`);

    assert.dom('*').hasText('false');
  });
});
