import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { render } from '@ember/test-helpers';
import hbs from 'htmlbars-inline-precompile';

function wait(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

module('Integration | Modifier | fix-code-block-aria', function (hooks) {
  setupRenderingTest(hooks);

  test('it adds role="region" to pre elements with aria-labelledby', async function (assert) {
    await render(hbs`
      <div {{fix-code-block-aria}}>
        <pre aria-labelledby="title-123">
          <code>console.log('hello');</code>
        </pre>
      </div>
    `);

    await wait(150);
    assert.dom('pre[aria-labelledby]').hasAttribute('role', 'region');
  });
});
