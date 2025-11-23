/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { render, click } from '@ember/test-helpers';
import hbs from 'htmlbars-inline-precompile';

function wait(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

module('Integration | Modifier | click-outside', function (hooks) {
  setupRenderingTest(hooks);

  test('it calls onClickOutside when clicking outside the element', async function (assert) {
    assert.expect(2);

    this.set('clickedOutside', false);
    this.set('handleClickOutside', () => {
      this.set('clickedOutside', true);
      assert.step('click-outside-called');
    });

    await render(hbs`
      <div data-test-container>
        <div
          {{click-outside
            enabled=true
            onClickOutside=this.handleClickOutside
          }}
          data-test-target
        >
          Target element
        </div>
      </div>
    `);

    // Click outside the target element
    await click('[data-test-container]');
    await wait(10);

    assert.verifySteps(['click-outside-called'], 'Click outside callback was triggered');
    assert.true(this.clickedOutside, 'Click outside state was updated');
  });

  test('it does not call onClickOutside when clicking inside the element', async function (assert) {
    assert.expect(1);

    this.set('clickedOutside', false);
    this.set('handleClickOutside', () => {
      this.set('clickedOutside', true);
      assert.step('click-outside-called');
    });

    await render(hbs`
      <div data-test-container>
        <div
          {{click-outside
            enabled=true
            onClickOutside=this.handleClickOutside
          }}
          data-test-target
        >
          Target element
        </div>
      </div>
    `);

    // Click inside the target element
    await click('[data-test-target]');
    await wait(10);

    assert.verifySteps([], 'Click outside callback was not triggered');
  });

  test('it does not call onClickOutside when disabled', async function (assert) {
    assert.expect(1);

    this.set('handleClickOutside', () => {
      assert.step('click-outside-called');
    });

    await render(hbs`
      <div data-test-container>
        <div
          {{click-outside
            enabled=false
            onClickOutside=this.handleClickOutside
          }}
          data-test-target
        >
          Target element
        </div>
      </div>
    `);

    // Click outside the target element
    await click('[data-test-container]');
    await wait(10);

    assert.verifySteps([], 'Click outside callback was not triggered when disabled');
  });

  test('it respects excluded elements', async function (assert) {
    assert.expect(2);

    let excludedElement;
    this.set('captureExcluded', (element) => {
      excludedElement = element;
    });

    this.set('handleClickOutside', () => {
      assert.step('click-outside-called');
    });

    await render(hbs`
      <div data-test-container>
        <div
          {{click-outside
            enabled=true
            excludeElements=(array this.excludedElement)
            onClickOutside=this.handleClickOutside
          }}
          data-test-target
        >
          Target element
        </div>
        <div {{element-ref this.captureExcluded}} data-test-excluded>
          Excluded element
        </div>
      </div>
    `);

    // Set the excluded element after render
    this.set('excludedElement', excludedElement);

    // Click on excluded element should not trigger callback
    await click('[data-test-excluded]');
    await wait(10);

    // Click on container (outside both target and excluded) should trigger callback
    await click('[data-test-container]');
    await wait(10);

    assert.verifySteps(['click-outside-called'], 'Only container click triggered callback');
    assert.step('test-completed');
  });

  test('it handles multiple excluded elements', async function (assert) {
    assert.expect(1);

    let excludedElement1, excludedElement2;
    this.set('captureExcluded1', (element) => {
      excludedElement1 = element;
    });
    this.set('captureExcluded2', (element) => {
      excludedElement2 = element;
    });

    this.set('handleClickOutside', () => {
      assert.step('click-outside-called');
    });

    await render(hbs`
      <div data-test-container>
        <div
          {{click-outside
            enabled=true
            excludeElements=(array this.excludedElement1 this.excludedElement2)
            onClickOutside=this.handleClickOutside
          }}
          data-test-target
        >
          Target element
        </div>
        <div {{element-ref this.captureExcluded1}} data-test-excluded-1>
          Excluded 1
        </div>
        <div {{element-ref this.captureExcluded2}} data-test-excluded-2>
          Excluded 2
        </div>
      </div>
    `);

    this.set('excludedElement1', excludedElement1);
    this.set('excludedElement2', excludedElement2);

    // Click on first excluded element
    await click('[data-test-excluded-1]');
    await wait(10);

    // Click on second excluded element
    await click('[data-test-excluded-2]');
    await wait(10);

    // Click on container should trigger callback
    await click('[data-test-container]');
    await wait(10);

    assert.verifySteps(['click-outside-called'], 'Only container click triggered callback');
  });

  test('it handles null excluded elements gracefully', async function (assert) {
    assert.expect(1);

    this.set('handleClickOutside', () => {
      assert.step('click-outside-called');
    });

    await render(hbs`
      <div data-test-container>
        <div
          {{click-outside
            enabled=true
            excludeElements=(array null undefined)
            onClickOutside=this.handleClickOutside
          }}
          data-test-target
        >
          Target element
        </div>
      </div>
    `);

    // Click outside should still work with null/undefined excluded elements
    await click('[data-test-container]');
    await wait(10);

    assert.verifySteps(['click-outside-called'], 'Callback triggered with null excluded elements');
  });

  test('it updates listeners when enabled state changes', async function (assert) {
    assert.expect(2);

    this.set('enabled', false);
    this.set('handleClickOutside', () => {
      assert.step('click-outside-called');
    });

    await render(hbs`
      <div data-test-container>
        <div
          {{click-outside
            enabled=this.enabled
            onClickOutside=this.handleClickOutside
          }}
          data-test-target
        >
          Target element
        </div>
      </div>
    `);

    // Click outside when disabled - should not trigger
    await click('[data-test-container]');
    await wait(10);

    // Enable and click outside - should trigger
    this.set('enabled', true);
    await click('[data-test-container]');
    await wait(10);

    assert.verifySteps(['click-outside-called'], 'Callback only triggered when enabled');
    assert.step('test-completed');
  });

  test('it cleans up listeners on destroy', async function (assert) {
    assert.expect(1);

    this.set('showElement', true);
    this.set('handleClickOutside', () => {
      assert.step('click-outside-called');
    });

    await render(hbs`
      <div data-test-container>
        {{#if this.showElement}}
          <div
            {{click-outside
              enabled=true
              onClickOutside=this.handleClickOutside
            }}
            data-test-target
          >
            Target element
          </div>
        {{/if}}
      </div>
    `);

    // Remove the element (should trigger cleanup)
    this.set('showElement', false);
    await wait(10);

    // Click outside should not trigger callback since element is destroyed
    await click('[data-test-container]');
    await wait(10);

    assert.verifySteps([], 'No callback triggered after element destruction');
  });

  test('it works with different event types', async function (assert) {
    assert.expect(1);

    this.set('handleClickOutside', (event) => {
      assert.equal(event.type, 'click', 'Event object passed to callback');
    });

    await render(hbs`
      <div data-test-container>
        <div
          {{click-outside
            enabled=true
            onClickOutside=this.handleClickOutside
          }}
          data-test-target
        >
          Target element
        </div>
      </div>
    `);

    await click('[data-test-container]');
    await wait(10);
  });
});
