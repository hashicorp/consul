import { module, skip } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { render } from '@ember/test-helpers';
import hbs from 'htmlbars-inline-precompile';

module('Integration | Helper | route-match', function(hooks) {
  setupRenderingTest(hooks);

  // Replace this with your real tests.
  skip('it renders', async function(assert) {
    this.set('inputValue', '1234');

    await render(hbs`{{route-match inputValue}}`);

    assert.equal(this.element.textContent.trim(), '1234');
  });
});
