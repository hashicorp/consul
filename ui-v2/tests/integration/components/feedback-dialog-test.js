import { module, skip, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { render } from '@ember/test-helpers';
import hbs from 'htmlbars-inline-precompile';

module('Integration | Component | feedback dialog', function(hooks) {
  setupRenderingTest(hooks);

  skip("it doesn't render anything when used inline");
  test('it renders', async function(assert) {
    // Set any properties with this.set('myProperty', 'value');
    // Handle any actions with this.on('myAction', function(val) { ... });

    await render(hbs`{{feedback-dialog}}`);

    assert.dom('*').hasText('');

    // Template block usage:
    await render(hbs`
      {{#feedback-dialog}}
        {{#block-slot 'success'}}
        {{/block-slot}}
        {{#block-slot 'error'}}
        {{/block-slot}}
      {{/feedback-dialog}}
    `);

    assert.dom('*').hasText('');
  });
});
