import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { render } from '@ember/test-helpers';
import hbs from 'htmlbars-inline-precompile';

module('Integration | Component | healthcheck list', function(hooks) {
  setupRenderingTest(hooks);

  test('it renders', async function(assert) {
    // Set any properties with this.set('myProperty', 'value');
    // Handle any actions with this.on('myAction', function(val) { ... });

    await render(hbs`{{healthcheck-list}}`);

    assert.dom('ul').exists({ count: 1 });

    // Template block usage:
    await render(hbs`
      {{#healthcheck-list}}
      {{/healthcheck-list}}
    `);

    assert.dom('ul').exists({ count: 1 });
  });
});
