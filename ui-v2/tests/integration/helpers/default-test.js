import { moduleForComponent, test } from 'ember-qunit';
import hbs from 'htmlbars-inline-precompile';

moduleForComponent('default', 'helper:default', {
  integration: true,
});

// Replace this with your real tests.
test('it renders', function(assert) {
  this.set('inputValue', '1234');

  this.render(hbs`{{default inputValue}}`);

  assert.equal(
    this.$()
      .text()
      .trim(),
    '1234'
  );
});
test('it renders the default value', function(assert) {
  this.set('inputValue', '');

  this.render(hbs`{{default inputValue '1234'}}`);

  assert.equal(
    this.$()
      .text()
      .trim(),
    '1234'
  );
});
