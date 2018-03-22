import { moduleForComponent, test } from 'ember-qunit';
import hbs from 'htmlbars-inline-precompile';

moduleForComponent('healthchecked-resource', 'Integration | Component | healthchecked resource', {
  integration: true,
});

test('it renders', function(assert) {
  // Set any properties with this.set('myProperty', 'value');
  // Handle any actions with this.on('myAction', function(val) { ... });

  this.render(hbs`{{healthchecked-resource}}`);

  assert.equal(
    this.$()
      .text()
      .trim(),
    ''
  );

  // Template block usage:
  this.render(hbs`
    {{#healthchecked-resource}}
      template block text
    {{/healthchecked-resource}}
  `);

  assert.equal(
    this.$()
      .text()
      .trim(),
    'template block text'
  );
});
