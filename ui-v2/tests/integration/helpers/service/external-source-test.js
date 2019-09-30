import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { render } from '@ember/test-helpers';
import hbs from 'htmlbars-inline-precompile';

module('helper:service/external-source', function(hooks) {
  setupRenderingTest(hooks);

  // Replace this with your real tests.
  test('it renders', async function(assert) {
    this.set('inputValue', { Meta: { 'external-source': 'consul' } });

    await render(hbs`{{service/external-source inputValue}}`);

    assert.dom('*').hasText('consul');
  });
  test('it renders prefixed', async function(assert) {
    this.set('inputValue', { Meta: { 'external-source': 'consul' } });

    await render(hbs`{{service/external-source inputValue prefix='external-source-'}}`);

    assert.dom('*').hasText('external-source-consul');
  });
});
