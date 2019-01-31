import { moduleForComponent, test } from 'ember-qunit';
import hbs from 'htmlbars-inline-precompile';

moduleForComponent('healthcheck-output', 'Integration | Component | healthcheck output', {
  integration: true,
});

test('it renders', function(assert) {
  // Set any properties with this.set('myProperty', 'value');
  // Handle any actions with this.on('myAction', function(val) { ... });

  this.render(hbs`{{healthcheck-output}}`);

  assert.notEqual(
    this.$()
      .text()
      .trim()
      .indexOf('Output'),
    -1
  );

  // Template block usage:
  this.render(hbs`
    {{#healthcheck-output}}{{/healthcheck-output}}
  `);

  assert.notEqual(
    this.$()
      .text()
      .trim()
      .indexOf('Output'),
    -1
  );
});
