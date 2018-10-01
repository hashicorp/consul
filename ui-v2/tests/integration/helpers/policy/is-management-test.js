import { moduleForComponent, test } from 'ember-qunit';
import hbs from 'htmlbars-inline-precompile';

moduleForComponent('policy/is-management', 'helper:policy/is-management', {
  integration: true,
});

// Replace this with your real tests.
test('it renders', function(assert) {
  this.set('inputValue', {});

  this.render(hbs`{{policy/is-management inputValue}}`);

  assert.equal(
    this.$()
      .text()
      .trim(),
    'false'
  );
});
