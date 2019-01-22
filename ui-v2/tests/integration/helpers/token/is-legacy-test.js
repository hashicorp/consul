import { moduleForComponent, test } from 'ember-qunit';
import hbs from 'htmlbars-inline-precompile';

moduleForComponent('token/is-legacy', 'helper:token/is-legacy', {
  integration: true,
});

// Replace this with your real tests.
test('it renders', function(assert) {
  this.set('inputValue', {});

  this.render(hbs`{{token/is-legacy inputValue}}`);

  assert.equal(
    this.$()
      .text()
      .trim(),
    'false'
  );
});
