
import { moduleForComponent, test } from 'ember-qunit';
import hbs from 'htmlbars-inline-precompile';

moduleForComponent('atob', 'helper:atob', {
  integration: true
});

// Replace this with your real tests.
test('it renders', function(assert) {
  this.set('inputValue', 'MTIzNA==');

  this.render(hbs`{{atob inputValue}}`);
  assert.equal(this.$().text().trim(), '1234');
});

