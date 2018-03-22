import { moduleForComponent, test } from 'ember-qunit';
import hbs from 'htmlbars-inline-precompile';

moduleForComponent('catalog-filter', 'Integration | Component | catalog filter', {
  integration: true,
});

test('it renders', function(assert) {
  // Set any properties with this.set('myProperty', 'value');
  // Handle any actions with this.on('myAction', function(val) { ... });

  this.render(hbs`{{catalog-filter}}`);

  assert.equal(
    this.$()
      .text()
      .trim(),
    ''
  );

  // Template block usage:
  this.render(hbs`
    {{#catalog-filter}}
      template block text
    {{/catalog-filter}}
  `);

  assert.equal(
    this.$()
      .text()
      .trim(),
    'template block text'
  );
});
