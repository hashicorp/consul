import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { render } from '@ember/test-helpers';
import hbs from 'htmlbars-inline-precompile';

module('Integration | Component | dom buffer flush', function(hooks) {
  setupRenderingTest(hooks);

  test('it renders', async function(assert) {
    // Set any properties with this.set('myProperty', 'value');
    // Handle any actions with this.on('myAction', function(val) { ... });

    await render(hbs`{{dom-buffer-flush}}`);

    assert.dom('*').hasText('');

    // Template block usage:
    await render(hbs`
      {{#dom-buffer-flush}}
        template block text
      {{/dom-buffer-flush}}
    `);

    assert.dom('*').hasText('template block text');
  });
});
