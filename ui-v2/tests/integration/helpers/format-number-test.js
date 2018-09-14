import { moduleForComponent, test } from 'ember-qunit';
import hbs from 'htmlbars-inline-precompile';

moduleForComponent('format-number', 'helper:format-number', {
  integration: true,
});

// Replace this with your real tests.
test('it renders', function(assert) {
  this.set('inputValue', 1234);

  this.render(hbs`{{format-number inputValue}}`);

  assert.equal(
    this.$()
      .text()
      .trim(),
    '1,234'
  );
});
