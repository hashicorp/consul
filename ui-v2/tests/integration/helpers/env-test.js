import { moduleForComponent, test } from 'ember-qunit';
import hbs from 'htmlbars-inline-precompile';

moduleForComponent('env', 'helper:env', {
  integration: true,
});

// Replace this with your real tests.
test('it renders', function(assert) {
  this.set('inputValue', 'CONSUL_COPYRIGHT_URL');

  this.render(hbs`{{env inputValue}}`);

  assert.equal(
    this.$()
      .text()
      .trim(),
    'https://www.hashicorp.com'
  );
});
