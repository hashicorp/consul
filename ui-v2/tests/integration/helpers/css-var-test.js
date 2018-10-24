import { moduleForComponent, test } from 'ember-qunit';
import hbs from 'htmlbars-inline-precompile';

moduleForComponent('css-var', 'helper:css-var', {
  integration: true,
});

// Replace this with your real tests.
test("it renders nothing if the variable doesn't exist", function(assert) {
  this.set('inputValue', '1234');

  this.render(hbs`{{css-var inputValue}}`);

  assert.equal(
    this.$()
      .text()
      .trim(),
    ''
  );
});
test("it renders a default if the variable doesn't exist", function(assert) {
  this.set('inputValue', '1234');

  this.render(hbs`{{css-var inputValue 'none'}}`);

  assert.equal(
    this.$()
      .text()
      .trim(),
    'none'
  );
});
