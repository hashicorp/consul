import { moduleForComponent, skip } from 'ember-qunit';
import hbs from 'htmlbars-inline-precompile';

moduleForComponent('panel-bar', 'helper:panel-bar', {
  integration: true,
});

// Replace this with your real tests.
skip('it renders', function(assert) {
  this.set('inputValue', '1234');

  this.render(hbs`{{panel-bar inputValue}}`);

  assert.equal(
    this.$()
      .text()
      .trim(),
    '1234'
  );
});
