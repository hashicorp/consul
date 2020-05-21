import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { render } from '@ember/test-helpers';
import { hbs } from 'ember-cli-htmlbars';

module('Integration | Helper | service/exists', function(hooks) {
  setupRenderingTest(hooks);

  // Replace this with your real tests.
  test('it renders', async function(assert) {
    this.set('inputValue', { InstanceCount: 3 });

    await render(hbs`{{service/exists inputValue}}`);

    assert.equal(this.element.textContent.trim(), 'true');
  });
});
