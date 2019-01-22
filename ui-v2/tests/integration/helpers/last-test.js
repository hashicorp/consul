import { moduleForComponent, test } from 'ember-qunit';
import hbs from 'htmlbars-inline-precompile';

moduleForComponent('last', 'helper:last', {
  integration: true,
});

// Replace this with your real tests.
test('it renders', function(assert) {
  this.set('inputValue', 'get-the-last-character/');

  this.render(hbs`{{last inputValue}}`);

  assert.equal(
    this.$()
      .text()
      .trim(),
    '/'
  );
});
