/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { render } from '@ember/test-helpers';
import hbs from 'htmlbars-inline-precompile';

module('Integration | Component | consul/role/form', function (hooks) {
  setupRenderingTest(hooks);

  hooks.beforeEach(function () {
    // Mock can helper for all tests
    this.owner.register('helper:can', () => true);

    // Set up common properties
    this.set('form', {});
    this.set('onCreate', () => {});
    this.set('onUpdate', () => {});
    this.set('onCancel', () => {});

    // Helper function to render component with common args
    this.renderRoleForm = async (extraArgs = {}) => {
      const args = {
        form: this.form,
        onCreate: this.onCreate,
        onUpdate: this.onUpdate,
        onCancel: this.onCancel,
        ...extraArgs,
      };

      return render(hbs`
        <Consul::Role::Form
          @form={{this.form}}
          @item={{this.item}}
          @create={{this.create}}
          @onCreate={{this.onCreate}}
          @onUpdate={{this.onUpdate}}
          @onCancel={{this.onCancel}}
        />
      `);
    };
  });

  test('it renders', async function (assert) {
    this.set('item', { Name: 'test-role' });

    await this.renderRoleForm();

    assert.dom('form').exists('Form element is rendered');
  });

  test('it shows Save button for create mode', async function (assert) {
    this.set('item', {
      Name: 'test-role',
      isPristine: false,
      isInvalid: false,
    });
    this.set('create', true);

    await this.renderRoleForm();

    assert.dom('button[type="submit"]').exists('Save button is rendered');
    assert.dom('button[type="submit"]').hasText('Save');
  });

  test('it shows Save button for edit mode', async function (assert) {
    this.set('item', {
      Name: 'test-role',
      ID: 'role-123',
      isInvalid: false,
    });
    this.set('create', false);

    await this.renderRoleForm();

    assert.dom('button[type="submit"]').exists('Save button is rendered for edit mode');
    assert.dom('[data-test-delete]').exists('Delete button is rendered for edit mode');
  });
});