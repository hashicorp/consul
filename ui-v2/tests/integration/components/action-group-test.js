import { moduleForComponent, test, skip } from 'ember-qunit';
import hbs from 'htmlbars-inline-precompile';

moduleForComponent('action-group', 'Integration | Component | action group', {
  integration: true,
});

skip("it doesn't render anything when used inline");
test('it renders', function(assert) {
  // Set any properties with this.set('myProperty', 'value');
  // Handle any actions with this.on('myAction', function(val) { ... });

  // this.render(hbs`{{action-group}}`);

  // assert.equal(
  //   this.$()
  //     .text()
  //     .trim(),
  //   ''
  // );

  // Template block usage:
  this.render(hbs`
    {{#action-group}}{{/action-group}}
  `);

  assert.notEqual(
    this.$()
      .text()
      .trim()
      .indexOf('Open'),
    -1
  );
});
