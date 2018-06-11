import { moduleForComponent, test } from 'ember-qunit';
import hbs from 'htmlbars-inline-precompile';

moduleForComponent('healthcheck-status', 'Integration | Component | healthcheck status', {
  integration: true,
});

test('it renders', function(assert) {
  // Set any properties with this.set('myProperty', 'value');
  // Handle any actions with this.on('myAction', function(val) { ... });

  this.render(hbs`{{healthcheck-status}}`);

  assert.notEqual(
    this.$()
      .text()
      .trim()
      .indexOf('Output'),
    -1
  );

  // Template block usage:
  this.render(hbs`
    {{#healthcheck-status}}{{/healthcheck-status}}
  `);

  assert.notEqual(
    this.$()
      .text()
      .trim()
      .indexOf('Output'),
    -1
  );
});
