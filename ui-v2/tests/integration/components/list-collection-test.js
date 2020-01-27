import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { render } from '@ember/test-helpers';
import hbs from 'htmlbars-inline-precompile';

module('Integration | Component | list collection', function(hooks) {
  setupRenderingTest(hooks);

  test('it renders', async function(assert) {
    // Set any properties with this.set('myProperty', 'value');
    // Handle any actions with this.on('myAction', function(val) { ... });

    await render(hbs`{{list-collection cell-layout=(fixed-grid-layout 800 50)}}`);

    assert.dom('*').hasText('');

    // Template block usage:
    await render(hbs`
      {{#list-collection cell-layout=(fixed-grid-layout 800 50)}}{{/list-collection}}
    `);

    assert.dom('*').hasText('');
  });
});
