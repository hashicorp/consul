import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { render } from '@ember/test-helpers';
import hbs from 'htmlbars-inline-precompile';

module('Integration | Component | copy button feedback', function(hooks) {
  setupRenderingTest(hooks);

  test('it renders', async function(assert) {
    // Set any properties with this.set('myProperty', 'value');
    // Handle any actions with this.on('myAction', function(val) { ... });

    await render(hbs`{{copy-button-feedback value='Click Me'}}`);

    assert.dom('*').hasText('Click Me');

    // Template block usage:
    await render(hbs`
      {{#copy-button-feedback}}Click Me{{/copy-button-feedback}}
    `);

    assert.dom('*').hasText('Click Me');
  });
});
