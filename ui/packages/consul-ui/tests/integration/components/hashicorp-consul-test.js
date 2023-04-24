import { module, skip } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import hbs from 'htmlbars-inline-precompile';

module('Integration | Component | hashicorp consul', function(hooks) {
  setupRenderingTest(hooks);

  skip('it renders', function(assert) {
    // Set any properties with this.set('myProperty', 'value');
    // Handle any actions with this.on('myAction', function(val) { ... });

    this.render(hbs`{{hashicorp-consul}}`);

    assert.dom('*').hasText('');

    // Template block usage:
    this.render(hbs`
      {{#hashicorp-consul}}
      {{/hashicorp-consul}}
    `);

    assert.dom('*').hasText('template block text');
  });
});
