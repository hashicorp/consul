/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { render, click, fillIn } from '@ember/test-helpers';
import { hbs } from 'ember-cli-htmlbars';

module('Integration | Component | super-select-with-create', function (hooks) {
  setupRenderingTest(hooks);

  hooks.beforeEach(function () {
    this.services = [
      { Name: 'web-frontend' },
      { Name: 'api-gateway' },
      { Name: 'user-service' },
      { Name: 'payment-service' },
      { Name: 'notification-service' },
      { Name: 'database-proxy' },
      { Name: 'redis-cache' },
    ];

    this.onChange = () => {};
  });

  test('it displays option to add item with default text', async function (assert) {
    await render(hbs`
      <SuperSelectWithCreate
        @options={{this.services}}
        @onChange={{this.onChange}}
        @searchField="Name"
      as |service|>
        {{service.Name}}
      </SuperSelectWithCreate>
    `);

    await click('.ember-power-select-trigger');
    await fillIn('.ember-power-select-search-input', 'new-service');

    const createOption = this.element.querySelector('.create-option');
    assert.dom(createOption).exists('Create option appears');
    assert.dom(createOption).containsText('Add "new-service"', 'Contains the search term');
  });

  test('it displays option to add item with custom text', async function (assert) {
    await render(hbs`
      <SuperSelectWithCreate
        @options={{this.services}}
        @onChange={{this.onChange}}
        @searchField="Name"
        @buildSuggestion="Use a Consul Service called '__TERM__'"
      as |service|>
        {{service.Name}}
      </SuperSelectWithCreate>
    `);

    await click('.ember-power-select-trigger');
    await fillIn('.ember-power-select-search-input', 'new-microservice');

    assert.dom('.create-option').hasText("Use a Consul Service called 'new-microservice'");
  });

  test('it positions create option correctly', async function (assert) {
    await render(hbs`
      <SuperSelectWithCreate
        @options={{this.services}}
        @onChange={{this.onChange}}
        @searchField="Name"
        @buildSuggestion="Create __TERM__"
        @showCreatePosition="bottom"
      as |service|>
        {{service.Name}}
      </SuperSelectWithCreate>
    `);

    await click('.ember-power-select-trigger');
    await fillIn('.ember-power-select-search-input', 'auth');

    const createOption = this.element.querySelector('.create-option');
    const allOptions = this.element.querySelectorAll('.ember-power-select-option');
    const createOptionParent = createOption.closest('.ember-power-select-option');

    assert.dom(createOption).hasText('Create auth');
    assert.strictEqual(
      allOptions[allOptions.length - 1],
      createOptionParent,
      'Create option appears at bottom'
    );
  });

  test('it shows create option at top by default', async function (assert) {
    await render(hbs`
      <SuperSelectWithCreate
        @options={{this.services}}
        @onChange={{this.onChange}}
        @searchField="Name"
        @buildSuggestion="Create __TERM__"
      as |service|>
        {{service.Name}}
      </SuperSelectWithCreate>
    `);

    await click('.ember-power-select-trigger');
    await fillIn('.ember-power-select-search-input', 'partial-match');

    const createOption = this.element.querySelector('.create-option');
    const allOptions = this.element.querySelectorAll('.ember-power-select-option');
    const createOptionParent = createOption.closest('.ember-power-select-option');

    assert.dom(createOption).exists('Create option appears');
    assert.strictEqual(
      allOptions[0],
      createOptionParent,
      'Create option appears at top by default'
    );
  });

  test('it executes the onChange callback when creating new option', async function (assert) {
    assert.expect(1);

    this.onChange = (newService) => {
      assert.strictEqual(
        newService.Name,
        'auth-service',
        'onChange called with correct value when creating'
      );
    };

    await render(hbs`
      <SuperSelectWithCreate
        @options={{this.services}}
        @onChange={{this.onChange}}
        @searchField="Name"
      as |service|>
        {{service.Name}}
      </SuperSelectWithCreate>
    `);

    await click('.ember-power-select-trigger');
    await fillIn('.ember-power-select-search-input', 'auth-service');
    await click('.create-option');
  });

  test('it filters options based on search term', async function (assert) {
    await render(hbs`
      <SuperSelectWithCreate
        @options={{this.services}}
        @onChange={{this.onChange}}
        @searchField="Name"
      as |service|>
        {{service.Name}}
      </SuperSelectWithCreate>
    `);

    await click('.ember-power-select-trigger');
    await fillIn('.ember-power-select-search-input', 'service');

    assert.dom('.ember-power-select-option').exists('Shows filtered options');
    assert.dom('.create-option').exists('Shows create option');
  });

  test('it hides create option when exact match exists', async function (assert) {
    await render(hbs`
      <SuperSelectWithCreate
        @options={{this.services}}
        @onChange={{this.onChange}}
        @searchField="Name"
      as |service|>
        {{service.Name}}
      </SuperSelectWithCreate>
    `);

    await click('.ember-power-select-trigger');
    await fillIn('.ember-power-select-search-input', 'web-frontend');

    assert.dom('.ember-power-select-option').exists('Shows exact match');
    assert.dom('.create-option').doesNotExist('Hides create option when exact match exists');
  });

  test('it calls onChange when existing option is selected', async function (assert) {
    assert.expect(1);

    this.onChange = (selectedService) => {
      assert.strictEqual(
        selectedService.Name,
        'api-gateway',
        'onChange called with selected service'
      );
    };

    await render(hbs`
      <SuperSelectWithCreate
        @options={{this.services}}
        @onChange={{this.onChange}}
        @searchField="Name"
      as |service|>
        {{service.Name}}
      </SuperSelectWithCreate>
    `);

    await click('.ember-power-select-trigger');

    const options = this.element.querySelectorAll('.ember-power-select-option');
    const apiGatewayOption = Array.from(options).find(
      (option) => option.textContent.trim() === 'api-gateway'
    );

    await click(apiGatewayOption || '.ember-power-select-option:nth-child(2)');
  });

  test('it handles empty and null options', async function (assert) {
    // Test empty array
    this.services = [];

    await render(hbs`
      <SuperSelectWithCreate
        @options={{this.services}}
        @onChange={{this.onChange}}
        @searchField="Name"
      as |service|>
        {{service.Name}}
      </SuperSelectWithCreate>
    `);

    await click('.ember-power-select-trigger');
    await fillIn('.ember-power-select-search-input', 'first-service');
    assert.dom('.create-option').exists('Shows create option with empty options');

    // Test null options
    this.services = null;

    await render(hbs`
      <SuperSelectWithCreate
        @options={{this.services}}
        @onChange={{this.onChange}}
        @searchField="Name"
      as |service|>
        {{service.Name}}
      </SuperSelectWithCreate>
    `);

    await click('.ember-power-select-trigger');
    await fillIn('.ember-power-select-search-input', 'second-service');
    assert.dom('.create-option').exists('Shows create option with null options');
  });

  test('it resets search term when dropdown closes', async function (assert) {
    await render(hbs`
      <SuperSelectWithCreate
        @options={{this.services}}
        @onChange={{this.onChange}}
        @searchField="Name"
      as |service|>
        {{service.Name}}
      </SuperSelectWithCreate>
    `);

    await click('.ember-power-select-trigger');
    await fillIn('.ember-power-select-search-input', 'test search');
    await click('.ember-power-select-trigger'); // Close dropdown
    await click('.ember-power-select-trigger'); // Reopen

    assert.dom('.ember-power-select-search-input').hasValue('', 'Search term reset after close');
  });

  test('it renders with helper and error text', async function (assert) {
    await render(hbs`
      <SuperSelectWithCreate
        @options={{this.services}}
        @onChange={{this.onChange}}
        @searchField="Name"
        @helperText="Select or create a service"
        @errorText="Service is required"
      as |service|>
        {{service.Name}}
      </SuperSelectWithCreate>
    `);

    assert.dom('.super-select-with-create').exists('Component renders');
  });

  test('it handles disabled state', async function (assert) {
    await render(hbs`
      <SuperSelectWithCreate
        @options={{this.services}}
        @onChange={{this.onChange}}
        @searchField="Name"
        @disabled={{true}}
      as |service|>
        {{service.Name}}
      </SuperSelectWithCreate>
    `);

    assert
      .dom('.ember-power-select-trigger')
      .hasAttribute('aria-disabled', 'true', 'Component is disabled');
  });
});
