import { moduleForComponent, test, skip } from 'ember-qunit';
import hbs from 'htmlbars-inline-precompile';

moduleForComponent('feedback-dialog', 'Integration | Component | feedback dialog', {
  integration: true,
});

skip("it doesn't render anything when used inline");
test('it renders', function(assert) {
  // Set any properties with this.set('myProperty', 'value');
  // Handle any actions with this.on('myAction', function(val) { ... });

  // this.render(hbs`{{feedback-dialog}}`);

  // assert.equal(
  //   this.$()
  //     .text()
  //     .trim(),
  //   ''
  // );

  // Template block usage:
  this.render(hbs`
    {{#feedback-dialog}}
      {{#block-slot 'success'}}
      {{/block-slot}}
      {{#block-slot 'error'}}
      {{/block-slot}}
    {{/feedback-dialog}}
  `);

  assert.equal(
    this.$()
      .text()
      .trim(),
    ''
  );
});
