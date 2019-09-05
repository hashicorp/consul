import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { render, find } from '@ember/test-helpers';
import hbs from 'htmlbars-inline-precompile';

module('Integration | Component | tomography graph', function(hooks) {
  setupRenderingTest(hooks);

  test('it renders', async function(assert) {
    // Set any properties with this.set('myProperty', 'value');
    // Handle any actions with this.on('myAction', function(val) { ... });

    await render(hbs`{{tomography-graph}}`);

    assert.ok(
      find('*')
        .textContent.trim()
        .indexOf('ms') !== -1
    );

    // Template block usage:
    await render(hbs`
      {{#tomography-graph}}{{/tomography-graph}}
    `);

    assert.ok(
      find('*')
        .textContent.trim()
        .indexOf('ms') !== -1
    );
  });
});
