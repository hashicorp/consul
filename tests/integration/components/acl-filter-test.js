import { moduleForComponent, test } from 'ember-qunit';
import hbs from 'htmlbars-inline-precompile';

moduleForComponent('acl-filter', 'Integration | Component | acl filter', {
  integration: true,
});

test('it renders', function(assert) {
  // Set any properties with this.set('myProperty', 'value');
  // Handle any actions with this.on('myAction', function(val) { ... });

  this.render(hbs`{{acl-filter}}`);

  assert.equal(
    this.$()
      .text()
      .trim(),
    ''
  );

  // Template block usage:
  this.render(hbs`
    {{#acl-filter}}
      template block text
    {{/acl-filter}}
  `);

  assert.equal(
    this.$()
      .text()
      .trim(),
    'template block text'
  );
});
