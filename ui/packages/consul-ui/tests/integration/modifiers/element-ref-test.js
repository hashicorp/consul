/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { render } from '@ember/test-helpers';
import hbs from 'htmlbars-inline-precompile';

module('Integration | Modifier | element-ref', function (hooks) {
  setupRenderingTest(hooks);

  test('it calls the callback with the element when inserted', async function (assert) {
    assert.expect(2);

    let capturedElement;
    this.set('captureElement', (element) => {
      capturedElement = element;
      assert.step('callback-called');
    });

    await render(hbs`
      <div {{element-ref this.captureElement}} data-test-target>
        Test content
      </div>
    `);

    assert.verifySteps(['callback-called'], 'Callback was called once');
    assert
      .dom(capturedElement)
      .hasAttribute('data-test-target', '', 'Correct element was captured');
  });

  test('it handles null callback gracefully', async function (assert) {
    this.set('nullCallback', null);

    await render(hbs`
      <div {{element-ref this.nullCallback}}>
        Test content
      </div>
    `);

    assert.dom('[data-test-target]').doesNotExist('No errors thrown with null callback');
  });

  test('it handles undefined callback gracefully', async function (assert) {
    await render(hbs`
      <div {{element-ref this.undefinedCallback}}>
        Test content
      </div>
    `);

    assert.dom('div').exists('No errors thrown with undefined callback');
  });

  test('it handles non-function callback gracefully', async function (assert) {
    this.set('notAFunction', 'not a function');

    await render(hbs`
      <div {{element-ref this.notAFunction}}>
        Test content
      </div>
    `);

    assert.dom('div').exists('No errors thrown with non-function callback');
  });

  test('it can capture multiple elements with different callbacks', async function (assert) {
    assert.expect(4);

    let firstElement, secondElement;

    this.set('captureFirst', (element) => {
      firstElement = element;
      assert.step('first-callback');
    });

    this.set('captureSecond', (element) => {
      secondElement = element;
      assert.step('second-callback');
    });

    await render(hbs`
      <div {{element-ref this.captureFirst}} data-test-first>First</div>
      <div {{element-ref this.captureSecond}} data-test-second>Second</div>
    `);

    assert.verifySteps(['first-callback', 'second-callback'], 'Both callbacks were called');
    assert.dom(firstElement).hasAttribute('data-test-first');
    assert.dom(secondElement).hasAttribute('data-test-second');
  });

  test('it works with different element types', async function (assert) {
    assert.expect(4);

    let capturedElements = [];
    this.set('captureElement', (element) => {
      capturedElements.push(element);
    });

    await render(hbs`
      <input {{element-ref this.captureElement}} data-test-input type="text" />
      <button {{element-ref this.captureElement}} data-test-button>Click</button>
      <span {{element-ref this.captureElement}} data-test-span>Text</span>
    `);

    assert.equal(capturedElements.length, 3, 'All elements were captured');
    assert.dom(capturedElements[0]).hasAttribute('data-test-input');
    assert.dom(capturedElements[1]).hasAttribute('data-test-button');
    assert.dom(capturedElements[2]).hasAttribute('data-test-span');
  });
});
