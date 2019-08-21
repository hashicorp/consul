import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { render } from '@ember/test-helpers';
import hbs from 'htmlbars-inline-precompile';

module('Integration | Component | intention filter', function(hooks) {
  setupRenderingTest(hooks);

  test('it renders', async function(assert) {
    // Set any properties with this.set('myProperty', 'value');
    // Handle any actions with this.on('myAction', function(val) { ... });

    await render(hbs`{{intention-filter}}`);

    assert.dom('*').hasText('Search');

    // // Template block usage:
    // this.render(hbs`
    //   {{#intention-filter}}
    //     template block text
    //   {{/intention-filter}}
    // `);

    // assert.equal(this.$().text().trim(), 'template block text');
  });
});
