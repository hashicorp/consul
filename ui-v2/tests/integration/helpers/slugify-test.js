import { moduleForComponent, test, skip } from 'ember-qunit';
import hbs from 'htmlbars-inline-precompile';

moduleForComponent('slugify', 'helper:slugify', {
  integration: true,
});

// Replace this with your real tests.
test('it renders', function(assert) {
  this.set('inputValue', 'Hi There');

  this.render(hbs`{{slugify inputValue}}`);

  assert.equal(
    this.$()
      .text()
      .trim(),
    'hi-there'
  );
});
skip("it copes with more values such as ' etc");
