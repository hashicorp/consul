/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { render } from '@ember/test-helpers';
import hbs from 'htmlbars-inline-precompile';
import Helper from '@ember/component/helper';

module('Integration | Component | consul/policy/form', function (hooks) {
  setupRenderingTest(hooks);

  const registerCan = (owner, allowed) => {
    owner.register(
      'helper:can',
      class extends Helper {
        compute([ability]) {
          return allowed.includes(ability);
        }
      }
    );
  };

  test('a user who can create policies but not tokens still gets a Save button', async function (assert) {
    registerCan(this.owner, ['create policies']);
    this.set('item', { Name: 'test-policy', isPristine: false, isInvalid: false });
    this.set('create', true);
    this.set('noop', () => {});

    await render(hbs`
      <Consul::Policy::Form
        @item={{this.item}}
        @create={{this.create}}
        @onCreate={{this.noop}}
        @onCancel={{this.noop}}
      />
    `);

    assert
      .dom('button[type="submit"]')
      .exists(
        'Save is shown for a user permitted to create policies, regardless of token permissions'
      );
  });

  test('a user who can create tokens but not policies does not get a Save button', async function (assert) {
    registerCan(this.owner, ['create tokens']);
    this.set('item', { Name: 'test-policy', isPristine: false, isInvalid: false });
    this.set('create', true);
    this.set('noop', () => {});

    await render(hbs`
      <Consul::Policy::Form
        @item={{this.item}}
        @create={{this.create}}
        @onCreate={{this.noop}}
        @onCancel={{this.noop}}
      />
    `);

    assert
      .dom('button[type="submit"]')
      .doesNotExist(
        'Save is not shown for a user who can only create tokens, not policies (the two capabilities are unrelated)'
      );
  });
});
