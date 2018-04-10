import { moduleForComponent, test } from 'ember-qunit';
import hbs from 'htmlbars-inline-precompile';

moduleForComponent('session-list', 'Integration | Component | session list', {
  integration: true,
});

test('it renders', function(assert) {
  // Set any properties with this.set('myProperty', 'value');
  // Handle any actions with this.on('myAction', function(val) { ... });

  this.render(hbs`{{session-list}}`);

  assert.ok(
    this.$()
      .text()
      .trim().indexOf('Name') !== -1
  );

  // Template block usage:
  this.render(hbs`
    {{#session-list}}{{/session-list}}
  `);

  assert.ok(
    this.$()
      .text()
      .trim().indexOf('Name') !== -1
  );
});
