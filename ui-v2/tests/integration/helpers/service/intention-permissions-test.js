import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { render } from '@ember/test-helpers';
import { hbs } from 'ember-cli-htmlbars';

module('Integration | Helper | service/intention-permissions', function(hooks) {
  setupRenderingTest(hooks);

  // TODO: Replace this with your real tests.
  test('it renders', async function(assert) {
    this.set('inputValue', {
      Intention: {
        Allowed: false,
        HasL7Permissions: true,
      },
    });

    await render(hbs`{{service/intention-permissions inputValue}}`);

    assert.equal(this.element.textContent.trim(), 'allow');
  });
});
