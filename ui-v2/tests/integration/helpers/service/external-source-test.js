import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { render } from '@ember/test-helpers';
import hbs from 'htmlbars-inline-precompile';

module('Integration | Helper | service/external-source', function(hooks) {
  setupRenderingTest(hooks);

  // Replace this with your real tests.
  test('it renders', async function(assert) {
    this.set('inputValue', { Meta: { 'external-source': 'consul' } });

    await render(hbs`{{service/external-source inputValue}}`);

    assert.equal(this.element.textContent.trim(), 'consul');
  });
  test('it renders prefixed', async function(assert) {
    this.set('inputValue', { Meta: { 'external-source': 'consul' } });

    await render(hbs`{{service/external-source inputValue prefix='external-source-'}}`);

    assert.equal(this.element.textContent.trim(), 'external-source-consul');
  });
});
