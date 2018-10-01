import { moduleForComponent, test } from 'ember-qunit';
import hbs from 'htmlbars-inline-precompile';

moduleForComponent('delete-confirmation', 'Integration | Component | delete confirmation', {
  integration: true,
});

test('it renders', function(assert) {
  // Set any properties with this.set('myProperty', 'value');
  // Handle any actions with this.on('myAction', function(val) { ... });

  this.render(hbs`{{delete-confirmation}}`);

  assert.equal(this.$('.type-delete').length, 1);

  // Template block usage:
  this.render(hbs`
    {{#delete-confirmation}}{{/delete-confirmation}}
  `);

  assert.equal(this.$('.type-delete').length, 1);
});
