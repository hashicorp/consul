import { moduleForComponent, test } from 'ember-qunit';
import hbs from 'htmlbars-inline-precompile';

moduleForComponent('healthcheck-list', 'Integration | Component | healthcheck list', {
  integration: true,
});

test('it renders', function(assert) {
  // Set any properties with this.set('myProperty', 'value');
  // Handle any actions with this.on('myAction', function(val) { ... });

  this.render(hbs`{{healthcheck-list}}`);

  assert.equal(this.$('ul').length, 1);

  // Template block usage:
  this.render(hbs`
    {{#healthcheck-list}}
    {{/healthcheck-list}}
  `);

  assert.equal(this.$('ul').length, 1);
});
