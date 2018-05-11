import { moduleForComponent, skip } from 'ember-qunit';
import hbs from 'htmlbars-inline-precompile';

moduleForComponent('is-href', 'helper:is-href', {
  integration: true,
});

// Replace this with your real tests.
skip('it renders', function(assert) {
  this.set('inputValue', '1234');

  this.render(hbs`{{is-href inputValue}}`);

  assert.equal(
    this.$()
      .text()
      .trim(),
    '1234'
  );
});
