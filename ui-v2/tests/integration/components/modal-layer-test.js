import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { render, findAll } from '@ember/test-helpers';
import hbs from 'htmlbars-inline-precompile';

module('Integration | Component | modal layer', function(hooks) {
  setupRenderingTest(hooks);

  test('it renders', async function(assert) {
    // Set any properties with this.set('myProperty', 'value');
    // Handle any actions with this.on('myAction', function(val) { ... });

    await render(hbs`{{modal-layer}}`);

    assert.ok(findAll('#modal_close').length === 1);

    // Template block usage:
    await render(hbs`
      {{#modal-layer}}
      {{/modal-layer}}
    `);
    assert.ok(findAll('#modal_close').length === 1);
  });
});
