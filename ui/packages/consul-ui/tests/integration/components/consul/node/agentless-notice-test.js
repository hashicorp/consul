import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import hbs from 'htmlbars-inline-precompile';
import { click, render } from '@ember/test-helpers';
import sinon from 'sinon';

module('Integration | Component | consul node agentless-notice', function (hooks) {
  setupRenderingTest(hooks);
  hooks.beforeEach(() => {
    const localStore = {};

    sinon.stub(window.localStorage, 'getItem').callsFake((key) => localStore[key]);
    sinon.stub(window.localStorage, 'setItem').callsFake((key, value) => (localStore[key] = value));
  });

  test('it does not display the notice if the filtered nodes are the same as the regular nodes', async function (assert) {
    this.set('nodes', [
      {
        Meta: {
          'synthetic-node': false,
        },
      },
    ]);

    this.set('filteredNodes', [
      {
        Meta: {
          'synthetic-node': false,
        },
      },
    ]);

    await render(
      hbs`<Consul::Node::AgentlessNotice @items={{this.nodes}} @filteredItems={{this.filteredNodes}} />`
    );
    assert.true(window.localStorage.getItem.called);
    assert
      .dom('[data-test-node-agentless-notice]')
      .doesNotExist(
        'The agentless notice should not display if the items are the same as the filtered items'
      );
  });

  test('it does display the notice when the filtered items are smaller then the regular items', async function (assert) {
    this.set('nodes', [
      {
        Meta: {
          'synthetic-node': true,
        },
      },
    ]);

    this.set('filteredNodes', []);

    await render(
      hbs`<Consul::Node::AgentlessNotice @items={{this.nodes}} @filteredItems={{this.filteredNodes}} />`
    );

    assert
      .dom('[data-test-node-agentless-notice]')
      .exists(
        'The agentless notice should display if their are less items then the filtered items'
      );

    await click('button');
    assert
      .dom('[data-test-node-agentless-notice]')
      .doesNotExist('The agentless notice be dismissed');
    assert.true(
      window.localStorage.setItem.calledOnceWith('consul-nodes-agentless-notice-dismissed', 'true'),
      "Set the key in localstorage to 'true'"
    );
  });

  test('it does not display if the localstorage key is already set to true', async function (assert) {
    this.set('nodes', [
      {
        Meta: {
          'synthetic-node': false,
        },
      },
    ]);

    this.set('filteredNodes', [
      {
        Meta: {
          'synthetic-node': false,
        },
      },
    ]);

    window.localStorage.setItem('consul-nodes-agentless-notice-dismissed-dc2', 'true');

    await render(
      hbs`<Consul::Node::AgentlessNotice @items={{this.nodes}} @filteredItems={{this.filteredNodes}} @dc="dc2" />`
    );

    assert.true(
      window.localStorage.getItem.calledOnceWith('consul-nodes-agentless-notice-dismissed-dc2')
    );

    assert
      .dom('[data-test-node-agentless-notice]')
      .doesNotExist(
        "The agentless notice should not display if the local storage key has already been set to 'true'"
      );
  });
});
