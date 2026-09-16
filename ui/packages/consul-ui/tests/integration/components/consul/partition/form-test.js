/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { render, fillIn } from '@ember/test-helpers';
import hbs from 'htmlbars-inline-precompile';

module('Integration | Component | consul/partition/form', function (hooks) {
  setupRenderingTest(hooks);

  hooks.beforeEach(function () {
    // Mock the ember-can `can`/`is` helpers for all tests, same approach as
    // tests/integration/components/consul/role/form-test.js.
    this.owner.register('helper:can', () => true);
    this.owner.register('helper:is', () => true);
  });

  test('typing in the Name and Description fields updates the item', async function (assert) {
    this.set('item', { Name: '', Description: '', Datacenter: 'dc-1' });

    await render(hbs`
      <Consul::Partition::Form
        @item={{this.item}}
        @dc="dc-1"
        @nspace="default"
        @partition="default"
      />
    `);

    await fillIn('[name="Name"]', 'my-partition');
    assert.strictEqual(
      this.item.Name,
      'my-partition',
      'typing in the Name field updates item.Name (HDS fields are one-way bound, so this needs an explicit input handler)'
    );

    await fillIn('[name="Description"]', 'a description');
    assert.strictEqual(
      this.item.Description,
      'a description',
      'typing in the Description field updates item.Description'
    );
  });
});
