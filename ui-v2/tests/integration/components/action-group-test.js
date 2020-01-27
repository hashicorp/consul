import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { render, find } from '@ember/test-helpers';
import hbs from 'htmlbars-inline-precompile';

module('Integration | Component | action group', function(hooks) {
  setupRenderingTest(hooks);

  test("it doesn't render anything when used inline", async function(assert) {
    await render(hbs`{{action-group}}`);

    assert.dom('*').hasText('');
  });
  test('it renders', async function(assert) {
    // Set any properties with this.set('myProperty', 'value');
    // Handle any actions with this.on('myAction', function(val) { ... });

    // this.render(hbs`{{action-group}}`);

    // assert.equal(
    //   this.$()
    //     .text()
    //     .trim(),
    //   ''
    // );

    // Template block usage:
    await render(hbs`
      {{#action-group}}{{/action-group}}
    `);

    assert.notEqual(
      find('*')
        .textContent.trim()
        .indexOf('Open'),
      -1
    );
  });
});
