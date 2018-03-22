import { moduleForComponent, test } from 'ember-qunit';
import hbs from 'htmlbars-inline-precompile';

moduleForComponent('datacenter-picker', 'Integration | Component | datacenter picker', {
  integration: true,
});

test('it renders', function(assert) {
  // Set any properties with this.set('myProperty', 'value');
  // Handle any actions with this.on('myAction', function(val) { ... });

  this.render(hbs`{{datacenter-picker}}`);

  assert.equal(
    this.$()
      .text()
      .trim(),
    ''
  );

  // Template block usage:
  this.render(hbs`
    {{#datacenter-picker}}
      template block text
    {{/datacenter-picker}}
  `);

  assert.equal(
    this.$()
      .text()
      .trim(),
    'template block text'
  );
});
