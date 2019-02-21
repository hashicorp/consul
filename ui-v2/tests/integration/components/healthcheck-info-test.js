import { moduleForComponent, test } from 'ember-qunit';
import hbs from 'htmlbars-inline-precompile';

moduleForComponent('healthcheck-info', 'Integration | Component | healthcheck info', {
  integration: true,
});

test('it renders', function(assert) {
  // Set any properties with this.set('myProperty', 'value');
  // Handle any actions with this.on('myAction', function(val) { ... });

  this.render(hbs`{{healthcheck-info}}`);

  assert.equal(this.$('dl').length, 1);

  // Template block usage:
  this.render(hbs`
    {{#healthcheck-info}}
    {{/healthcheck-info}}
  `);
  assert.equal(this.$('dl').length, 1);
});
