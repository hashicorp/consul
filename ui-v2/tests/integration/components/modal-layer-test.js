import { moduleForComponent, test } from 'ember-qunit';
import hbs from 'htmlbars-inline-precompile';

moduleForComponent('modal-layer', 'Integration | Component | modal layer', {
  integration: true,
});

test('it renders', function(assert) {
  // Set any properties with this.set('myProperty', 'value');
  // Handle any actions with this.on('myAction', function(val) { ... });

  this.render(hbs`{{modal-layer}}`);

  assert.ok(this.$('#modal_close').length === 1);

  // Template block usage:
  this.render(hbs`
    {{#modal-layer}}
    {{/modal-layer}}
  `);
  assert.ok(this.$('#modal_close').length === 1);
});
