import { moduleForComponent, test } from 'ember-qunit';
import hbs from 'htmlbars-inline-precompile';

moduleForComponent('secret-button', 'Integration | Component | secret button', {
  integration: true,
});

test('it renders', function(assert) {
  // Set any properties with this.set('myProperty', 'value');
  // Handle any actions with this.on('myAction', function(val) { ... });

  this.render(hbs`{{secret-button}}`);

  assert.ok(
    this.$()
      .text()
      .trim()
      .indexOf('Reveal') !== -1
  );

  // Template block usage:
  this.render(hbs`
    {{#secret-button}}
    {{/secret-button}}
  `);

  assert.ok(
    this.$()
      .text()
      .trim()
      .indexOf('Reveal') !== -1
  );
});
