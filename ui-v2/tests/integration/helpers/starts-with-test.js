import { moduleForComponent, test } from 'ember-qunit';
import hbs from 'htmlbars-inline-precompile';

moduleForComponent('starts-with', 'helper:starts-with', {
  integration: true,
});

// Replace this with your real tests.
test('it renders', function(assert) {
  this.set('inputValue', '1234');

  this.render(hbs`{{starts-with inputValue}}`);

  assert.equal(
    this.$()
      .text()
      .trim(),
    'false'
  );
});
