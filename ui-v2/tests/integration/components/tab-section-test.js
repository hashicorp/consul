import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { render } from '@ember/test-helpers';
import hbs from 'htmlbars-inline-precompile';

module('Integration | Component | tab section', function(hooks) {
  setupRenderingTest(hooks);

  test('it renders', async function(assert) {
    // Set any properties with this.set('myProperty', 'value');
    // Handle any actions with this.on('myAction', function(val) { ... });

    await render(hbs`{{tab-section}}`);

    assert.dom('*').hasText('');

    // Template block usage:
    await render(hbs`
      {{#tab-section}}
        template block text
      {{/tab-section}}
    `);

    assert.dom('*').hasText('template block text');
  });
});
