import { module, skip } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { render } from '@ember/test-helpers';
import hbs from 'htmlbars-inline-precompile';

module('Integration | Helper | href-mut', function(hooks) {
  setupRenderingTest(hooks);

  // Replace this with your real tests.
  skip('it renders', async function(assert) {
    await render(hbs`{{href-mut (hash dc=dc-1)}}`);

    assert.equal(this.element.textContent.trim(), '');
  });
});
