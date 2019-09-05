import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { render, find } from '@ember/test-helpers';
import hbs from 'htmlbars-inline-precompile';

module('Integration | Component | phrase editor', function(hooks) {
  setupRenderingTest(hooks);

  hooks.beforeEach(function() {
    this.actions = {};
    this.send = (actionName, ...args) => this.actions[actionName].apply(this, args);
  });

  test('it renders a phrase', async function(assert) {
    this.set('value', ['phrase']);
    await render(hbs`{{phrase-editor value=value}}`);
    assert.notEqual(
      find('*')
        .textContent.trim()
        .indexOf('phrase'),
      -1
    );
  });
  test('it calls onchange when a phrase is removed by clicking the phrase remove button and refocuses', async function(assert) {
    assert.expect(3);
    this.set('value', ['phrase']);
    this.actions.change = function(e) {
      assert.equal(e.target.value.length, 0);
    };
    await render(hbs`{{phrase-editor value=value onchange=(action 'change')}}`);
    assert.notEqual(
      find('*')
        .textContent.trim()
        .indexOf('phrase'),
      -1
    );
    const $input = this.$('input');
    const $button = this.$('button');
    $button.trigger('click');
    assert.equal(document.activeElement, $input.get(0));
  });
  test('it calls onchange when a phrase is added', async function(assert) {
    assert.expect(1);
    this.actions.change = function(e) {
      assert.equal(e.target.value.length, 2);
    };
    this.set('value', ['phrase']);
    await render(hbs`{{phrase-editor value=value onchange=(action 'change')}}`);
    const $input = this.$('input');
    $input.get(0).value = 'phrase 2';
    $input.trigger('input');
    $input.trigger('search');
  });
});
