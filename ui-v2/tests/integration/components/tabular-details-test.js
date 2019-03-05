import { moduleForComponent, test } from 'ember-qunit';
import hbs from 'htmlbars-inline-precompile';

moduleForComponent('tabular-details', 'Integration | Component | tabular details', {
  integration: true,
});

test('it renders', function(assert) {
  // Set any properties with this.set('myProperty', 'value');
  // Handle any actions with this.on('myAction', function(val) { ... });

  this.render(hbs`{{tabular-details}}`);

  assert.equal(
    this.$()
      .text()
      .trim(),
    'Actions'
  );

  // Template block usage:
  this.render(hbs`
    {{#tabular-details}}
    {{/tabular-details}}
  `);

  assert.equal(
    this.$()
      .text()
      .trim(),
    'Actions'
  );
});
