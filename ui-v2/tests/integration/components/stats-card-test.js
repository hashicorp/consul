import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { render, find } from '@ember/test-helpers';
import hbs from 'htmlbars-inline-precompile';

module('Integration | Component | stats card', function(hooks) {
  setupRenderingTest(hooks);

  test('it renders', async function(assert) {
    // Set any properties with this.set('myProperty', 'value');
    // Handle any actions with this.on('myAction', function(val) { ... });

    // Template block usage:
    await render(hbs`
      {{#stats-card}}
        {{#block-slot 'icon'}}icon{{/block-slot}}
        {{#block-slot 'mini-stat'}}mini-stat{{/block-slot}}
        {{#block-slot 'header'}}header{{/block-slot}}
        {{#block-slot 'body'}}body{{/block-slot}}
      {{/stats-card}}
    `);
    ['icon', 'mini-stat', 'header'].forEach(item => {
      assert.ok(
        find('header')
          .textContent.trim()
          .indexOf(item) !== -1
      );
    });
    assert.ok(
      find('*')
        .textContent.trim()
        .indexOf('body') !== -1
    );
  });
});
