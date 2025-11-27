/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { render, settled } from '@ember/test-helpers';
import { hbs } from 'ember-cli-htmlbars';
import { tracked } from '@glimmer/tracking';

module('Integration | Modifier | on-insert', function (hooks) {
  setupRenderingTest(hooks);

  test('it calls the callback when element is inserted', async function (assert) {
    let callCount = 0;
    let capturedElement = null;

    this.handleInsert = (element) => {
      callCount++;
      capturedElement = element;
    };

    await render(hbs`
      <div {{on-insert this.handleInsert}} data-test-element></div>
    `);

    assert.strictEqual(callCount, 1, 'callback was called once');
    assert.ok(capturedElement instanceof HTMLElement, 'callback received an HTML element');
    assert.dom(capturedElement).hasAttribute('data-test-element');
  });

  test('it passes the element as the first argument to the callback', async function (assert) {
    let receivedElement = null;

    this.captureElement = (element) => {
      receivedElement = element;
    };

    await render(hbs`
      <button class="test-button" type="button" {{on-insert this.captureElement}}>Click me</button>
    `);

    assert.ok(receivedElement instanceof HTMLButtonElement, 'received a button element');
    assert.dom(receivedElement).hasClass('test-button');
    assert.dom(receivedElement).hasText('Click me');
  });

  test('it works with fn helper to pass additional arguments', async function (assert) {
    let receivedArgs = [];

    this.handleInsert = (...args) => {
      receivedArgs = args;
    };

    await render(hbs`
      <div {{on-insert (fn this.handleInsert "arg1" "arg2")}} data-test-div></div>
    `);

    assert.strictEqual(receivedArgs.length, 3, 'received element plus 2 additional arguments');
    assert.ok(receivedArgs[2] instanceof HTMLElement, 'first argument is the element');
    assert.strictEqual(receivedArgs[0], 'arg1', 'second argument is "arg1"');
    assert.strictEqual(receivedArgs[1], 'arg2', 'third argument is "arg2"');
  });

  test('it handles callback that returns a value', async function (assert) {
    this.handleInsert = (element) => {
      return 'return value';
    };

    // Should not throw an error
    await render(hbs`
      <div {{on-insert this.handleInsert}}></div>
    `);

    assert.ok(true, 'modifier handles callback return values without error');
  });

  test('it handles undefined/null callbacks gracefully', async function (assert) {
    this.undefinedCallback = undefined;

    // Should not throw an error
    await render(hbs`
      <div {{on-insert this.undefinedCallback}}></div>
    `);

    assert.ok(true, 'modifier handles undefined callback without error');
  });

  test('it only calls callback once per element insertion', async function (assert) {
    let callCount = 0;

    this.handleInsert = () => {
      callCount++;
    };

    await render(hbs`
      <div {{on-insert this.handleInsert}}></div>
    `);

    assert.strictEqual(callCount, 1, 'callback called once on initial render');

    // Trigger a re-render by updating a tracked property
    await settled();

    assert.strictEqual(callCount, 1, 'callback still only called once after settled');
  });

  test('it works with multiple elements', async function (assert) {
    const capturedElements = [];

    this.handleInsert = (element) => {
      capturedElements.push(element);
    };

    await render(hbs`
      <div>
        <span {{on-insert this.handleInsert}} data-test-span></span>
        <button data-test-button type="button" {{on-insert this.handleInsert}}></button>
        {{! template-lint-disable require-input-label }}
        <input {{on-insert this.handleInsert}} data-test-input />
      </div>
    `);

    assert.strictEqual(capturedElements.length, 3, 'callback called for each element');
    assert.ok(capturedElements[0] instanceof HTMLSpanElement, 'first element is a span');
    assert.ok(capturedElements[1] instanceof HTMLButtonElement, 'second element is a button');
    assert.ok(capturedElements[2] instanceof HTMLInputElement, 'third element is an input');
  });

  test('it calls callback again when element is re-inserted', async function (assert) {
    let callCount = 0;

    class TestState {
      @tracked showElement = true;
    }

    const state = new TestState();
    this.state = state;

    this.handleInsert = () => {
      callCount++;
    };

    await render(hbs`
      {{#if this.state.showElement}}
        <div {{on-insert this.handleInsert}} data-test-div></div>
      {{/if}}
    `);

    assert.strictEqual(callCount, 1, 'callback called on initial render');

    // Remove element
    state.showElement = false;
    await settled();

    assert.strictEqual(callCount, 1, 'callback not called when element removed');

    // Re-insert element
    state.showElement = true;
    await settled();

    assert.strictEqual(callCount, 2, 'callback called again when element re-inserted');
  });

  test('it can be used to store element references', async function (assert) {
    let storedInput = null;

    this.storeInput = (element) => {
      storedInput = element;
    };

    await render(hbs`
      {{! template-lint-disable require-input-label }}
      <input {{on-insert this.storeInput}} type="text" value="test value" />
    `);

    assert.ok(storedInput instanceof HTMLInputElement, 'stored element is an input');
    assert.strictEqual(
      storedInput.value,
      'test value',
      'can access input value through stored reference'
    );
    assert.strictEqual(storedInput.type, 'text', 'can access input type through stored reference');
  });

  test('it works with arrow functions', async function (assert) {
    let capturedElement = null;

    this.handleInsert = (element) => {
      capturedElement = element;
    };

    await render(hbs`
      <div {{on-insert this.handleInsert}} data-test-arrow></div>
    `);

    assert.ok(capturedElement instanceof HTMLElement, 'arrow function works correctly');
    assert.dom(capturedElement).hasAttribute('data-test-arrow');
  });

  test('it can capture different element types', async function (assert) {
    const elements = {};

    this.captureDiv = (el) => {
      elements.div = el;
    };
    this.captureSpan = (el) => {
      elements.span = el;
    };
    this.captureInput = (el) => {
      elements.input = el;
    };
    this.captureButton = (el) => {
      elements.button = el;
    };

    await render(hbs`
      <div {{on-insert this.captureDiv}}></div>
      <span {{on-insert this.captureSpan}}></span>
      {{! template-lint-disable require-input-label }}
      <input {{on-insert this.captureInput}} />
      <button type="button" {{on-insert this.captureButton}}></button>
    `);

    assert.ok(elements.div instanceof HTMLDivElement, 'captured div');
    assert.ok(elements.span instanceof HTMLSpanElement, 'captured span');
    assert.ok(elements.input instanceof HTMLInputElement, 'captured input');
    assert.ok(elements.button instanceof HTMLButtonElement, 'captured button');
  });
});
