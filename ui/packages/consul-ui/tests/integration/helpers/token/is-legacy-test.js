import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { render } from '@ember/test-helpers';
import hbs from 'htmlbars-inline-precompile';

module('Integration | Helper | token/is-legacy', function (hooks) {
  setupRenderingTest(hooks);

  // Replace this with your real tests.
  test('it renders', async function (assert) {
    this.set('inputValue', {});

    await render(hbs`{{token/is-legacy inputValue}}`);

    assert.equal(this.element.textContent.trim(), 'false');
  });
});
