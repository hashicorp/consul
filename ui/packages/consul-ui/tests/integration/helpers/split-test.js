import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { render } from '@ember/test-helpers';
import hbs from 'htmlbars-inline-precompile';

module('Integration | Helper | split', function (hooks) {
  setupRenderingTest(hooks);

  // Replace this with your real tests.
  test('it renders', async function (assert) {
    this.set('inputValue', 'a,string,split,by,a,comma');

    await render(hbs`{{split inputValue}}`);

    assert.equal(this.element.textContent.trim(), 'a,string,split,by,a,comma');
  });
});
