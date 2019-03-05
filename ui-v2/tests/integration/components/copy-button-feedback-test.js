import { moduleForComponent, test } from 'ember-qunit';
import hbs from 'htmlbars-inline-precompile';

moduleForComponent('copy-button-feedback', 'Integration | Component | copy button feedback', {
  integration: true,
});

test('it renders', function(assert) {
  // Set any properties with this.set('myProperty', 'value');
  // Handle any actions with this.on('myAction', function(val) { ... });

  this.render(hbs`{{copy-button-feedback value='Click Me'}}`);

  assert.equal(
    this.$()
      .text()
      .trim(),
    'Click Me'
  );

  // Template block usage:
  this.render(hbs`
    {{#copy-button-feedback}}Click Me{{/copy-button-feedback}}
  `);

  assert.equal(
    this.$()
      .text()
      .trim(),
    'Click Me'
  );
});
