import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { render } from '@ember/test-helpers';
import hbs from 'htmlbars-inline-precompile';

module('Integration | Component | service identity', function(hooks) {
  setupRenderingTest(hooks);

  test('it renders', async function(assert) {
    // Set any properties with this.set('myProperty', 'value');
    // Handle any actions with this.on('myAction', function(val) { ... });

    await render(hbs`{{service-identity}}`);

    assert.ok(this.element.textContent.trim().indexOf('service_prefix') !== -1);

    // Template block usage:
    await render(hbs`
      {{#service-identity}}{{/service-identity}}
    `);

    assert.ok(this.element.textContent.trim().indexOf('service_prefix') !== -1);
  });
});
