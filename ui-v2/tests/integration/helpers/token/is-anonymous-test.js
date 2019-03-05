import { moduleForComponent, test } from 'ember-qunit';
import hbs from 'htmlbars-inline-precompile';

moduleForComponent('token/is-anonymous', 'helper:token/is-anonymous', {
  integration: true,
});

// Replace this with your real tests.
test('it renders', function(assert) {
  this.set('inputValue', { AccessorID: '00000' });

  this.render(hbs`{{token/is-anonymous inputValue}}`);

  assert.equal(
    this.$()
      .text()
      .trim(),
    'false'
  );
});
