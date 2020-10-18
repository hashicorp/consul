import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { render } from '@ember/test-helpers';
import hbs from 'htmlbars-inline-precompile';

module('Integration | Helper | format-number', function(hooks) {
  setupRenderingTest(hooks);

  test('it renders a formatted number when passed a number', async function(assert) {
    this.set('inputValue', 1234);

    await render(hbs`{{format-number inputValue}}`);

    assert.equal(this.element.textContent.trim(), '1,234');
  });
});
