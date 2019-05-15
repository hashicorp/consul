import { moduleForComponent, test } from 'ember-qunit';
import hbs from 'htmlbars-inline-precompile';

moduleForComponent('policy/typeof', 'helper:policy/typeof', {
  integration: true,
});

// Replace this with your real tests.
test('it renders', function(assert) {
  this.set('inputValue', '1234');

  this.render(hbs`{{policy/typeof inputValue}}`);

  assert.equal(
    this.$()
      .text()
      .trim(),
    'role'
  );
});
