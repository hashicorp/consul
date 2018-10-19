import { moduleForComponent, test } from 'ember-qunit';
import hbs from 'htmlbars-inline-precompile';

moduleForComponent('policy/datacenters', 'helper:policy/datacenters', {
  integration: true,
});

// Replace this with your real tests.
test('it renders', function(assert) {
  this.set('inputValue', {});

  this.render(hbs`{{policy/datacenters inputValue}}`);

  assert.equal(
    this.$()
      .text()
      .trim(),
    'All'
  );
});
