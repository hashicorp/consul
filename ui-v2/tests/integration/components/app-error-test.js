import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { render } from '@ember/test-helpers';
import hbs from 'htmlbars-inline-precompile';

module('Integration | Component | app-error', function(hooks) {
  setupRenderingTest(hooks);

  test('it renders', async function(assert) {
    // Set any properties with this.set('myProperty', 'value');
    // Handle any actions with this.set('myAction', function(val) { ... });

    await render(hbs`{{app-error}}`);

    assert.equal([...this.element.querySelectorAll('[data-test-error]')].length, 1);

    // Template block usage:
    await render(hbs`
      {{#app-error}}{{/app-error}}
    `);

    assert.equal([...this.element.querySelectorAll('[data-test-error]')].length, 1);
  });
});
