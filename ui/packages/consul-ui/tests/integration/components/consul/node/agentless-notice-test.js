/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import hbs from 'htmlbars-inline-precompile';
import { click, render } from '@ember/test-helpers';

module('Integration | Component | consul node agentless-notice', function (hooks) {
  setupRenderingTest(hooks);

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
  });

  test('it does not display if the localstorage key is already set to true', async function (assert) {
    this.set('nodes', [
      {
        Meta: {
          'synthetic-node': false,
        },
      },
    ]);

    this.set('filteredNodes', []);

    const localStorage = this.owner.lookup('service:local-storage');
    localStorage.storage.seed({
      notices: ['nodes-agentless-dismissed-partition'],
    });

    await render(
      hbs`<Consul::Node::AgentlessNotice @items={{this.nodes}} @filteredItems={{this.filteredNodes}} @postfix="partition" />`
    );

    assert
      .dom('[data-test-node-agentless-notice]')
      .doesNotExist(
        'The agentless notice should not display if the dismissal has already been stored in local storage'
      );
  });
});
