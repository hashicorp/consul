import { moduleForComponent, test } from 'ember-qunit';
import hbs from 'htmlbars-inline-precompile';

moduleForComponent('service-identity', 'Integration | Component | service identity', {
  integration: true,
});

test('it renders', function(assert) {
  // Set any properties with this.set('myProperty', 'value');
  // Handle any actions with this.on('myAction', function(val) { ... });

  this.render(hbs`{{service-identity}}`);

  assert.ok(
    this.$()
      .text()
      .trim()
      .indexOf('service_prefix') !== -1,
    ''
  );

  // Template block usage:
  this.render(hbs`
    {{#service-identity}}{{/service-identity}}
  `);

  assert.ok(
    this.$()
      .text()
      .trim()
      .indexOf('service_prefix') !== -1,
    ''
  );
});
