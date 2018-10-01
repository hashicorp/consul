import { moduleForComponent, test } from 'ember-qunit';
import hbs from 'htmlbars-inline-precompile';

moduleForComponent('modal-dialog', 'Integration | Component | modal dialog', {
  integration: true,
});

test('it renders', function(assert) {
  // Set any properties with this.set('myProperty', 'value');
  // Handle any actions with this.on('myAction', function(val) { ... });

  this.render(hbs`{{modal-dialog}}`);

  assert.ok(
    this.$()
      .text()
      .trim()
      .indexOf('Close') !== -1
  );

  // Template block usage:
  this.render(hbs`
    {{#modal-dialog}}
    {{/modal-dialog}}
  `);

  assert.ok(
    this.$()
      .text()
      .trim()
      .indexOf('Close') !== -1
  );
});
