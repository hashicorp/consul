import { module, skip } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { render } from '@ember/test-helpers';
import { hbs } from 'ember-cli-htmlbars';

module('Integration | Component | catalog-toolbar', function(hooks) {
  setupRenderingTest(hooks);

  skip('it renders', async function(assert) {
    // Set any properties with this.set('myProperty', 'value');
    // Handle any actions with this.set('myAction', function(val) { ... });

    await render(hbs`<CatalogToolbar />`);

    assert.equal(this.element.querySelector('form').length, 1);

    // Template block usage:
    await render(hbs`
      <CatalogToolbar>
        template block text
      </CatalogToolbar>
    `);

    assert.equal(this.element.textContent.trim(), 'template block text');
  });
});
