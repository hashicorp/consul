import { moduleForComponent, test } from 'ember-qunit';
import hbs from 'htmlbars-inline-precompile';

moduleForComponent('format-number', 'helper:format-number', {
  integration: true,
});

test('it renders a formatted number when passed a number', function(assert) {
  this.set('inputValue', 1234);

  this.render(hbs`{{format-number inputValue}}`);

  assert.equal(
    this.$()
      .text()
      .trim(),
    '1,234'
  );
});
