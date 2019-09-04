import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { render } from '@ember/test-helpers';
import hbs from 'htmlbars-inline-precompile';

module('Integration | Component | sort control', function(hooks) {
  setupRenderingTest(hooks);

  hooks.beforeEach(function() {
    this.actions = {};
    this.send = (actionName, ...args) => this.actions[actionName].apply(this, args);
  });

  test('it renders', async function(assert) {
    // Set any properties with this.set('myProperty', 'value');
    // Handle any actions with this.on('myAction', function(val) { ... });

    await render(hbs`{{sort-control}}`);

    assert.dom('*').hasText('');

    // Template block usage:
    await render(hbs`
      {{#sort-control}}
        template block text
      {{/sort-control}}
    `);

    assert.dom('*').hasText('template block text');
  });
  test('it changes direction and calls onchange when clicked/activated', async function(assert) {
    assert.expect(2);
    let count = 0;
    this.actions.change = e => {
      if (count === 0) {
        assert.equal(e.target.value, 'sort:desc');
      } else {
        assert.equal(e.target.value, 'sort:asc');
      }
      count++;
    };
    await render(hbs`{{sort-control checked=true value='sort' onchange=(action 'change')}}`);
    const $label = this.$('label');
    $label.trigger('click');
    $label.trigger('click');
  });
});
