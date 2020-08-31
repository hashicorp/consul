import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { render } from '@ember/test-helpers';
import hbs from 'htmlbars-inline-precompile';

module('Integration | Helper | env', function(hooks) {
  setupRenderingTest(hooks);

  // Replace this with your real tests.
  test('it renders', async function(assert) {
    this.set('inputValue', 'CONSUL_COPYRIGHT_URL');

    await render(hbs`{{env inputValue}}`);

    assert.equal(this.element.textContent.trim(), 'https://www.hashicorp.com');
  });
});
