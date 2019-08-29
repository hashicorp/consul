import { moduleForComponent, test } from 'ember-qunit';
import hbs from 'htmlbars-inline-precompile';

moduleForComponent('object-entries', 'helper:object-entries', {
  integration: true,
});

// Replace this with your real tests.
test('it renders', function(assert) {
  this.set('inputValue', '1234');

  this.render(hbs`{{object-entries inputValue}}`);

  assert.equal(
    this.$()
      .text()
      .trim(),
    Object.entries('1234').toString()
  );
});
