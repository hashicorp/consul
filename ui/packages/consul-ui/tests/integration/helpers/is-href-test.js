import { module, skip } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import hbs from 'htmlbars-inline-precompile';

module('Integration | Helper | is-href', function(hooks) {
  setupRenderingTest(hooks);

  // Replace this with your real tests.
  skip('it renders', function(assert) {
    this.set('inputValue', '1234');

    this.render(hbs`{{is-href inputValue}}`);

    assert.equal(this.element.textContent.trim(), '1234');
  });
});
