/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { render, settled, waitUntil } from '@ember/test-helpers';
import { hbs } from 'ember-cli-htmlbars';
import { tracked } from '@glimmer/tracking';

module('Integration | Modifier | a11y-dialog', function (hooks) {
  setupRenderingTest(hooks);

  test('it creates an a11y-dialog instance', async function (assert) {
    let dialogInstance = null;

    this.handleSetup = (dialog) => {
      dialogInstance = dialog;
    };

    await render(hbs`
      <div {{a11y-dialog onSetup=this.handleSetup}} data-a11y-dialog>
        <div role="dialog">
          <button data-a11y-dialog-hide>Close</button>
          <div>Dialog content</div>
        </div>
      </div>
    `);

    assert.ok(dialogInstance, 'dialog instance was created');
    assert.strictEqual(typeof dialogInstance.show, 'function', 'dialog has show method');
    assert.strictEqual(typeof dialogInstance.hide, 'function', 'dialog has hide method');
    assert.strictEqual(typeof dialogInstance.destroy, 'function', 'dialog has destroy method');
  });

  test('it calls onShow callback when dialog is shown', async function (assert) {
    let showCallCount = 0;
    let showTarget = null;
    let dialogInstance = null;

    this.handleSetup = (dialog) => {
      dialogInstance = dialog;
    };

    this.handleShow = ({ target }) => {
      showCallCount++;
      showTarget = target;
    };

    await render(hbs`
      <div {{a11y-dialog onSetup=this.handleSetup onShow=this.handleShow}} data-test-dialog>
        <div role="dialog">
          <button data-a11y-dialog-hide>Close</button>
          <div>Dialog content</div>
        </div>
      </div>
    `);

    assert.strictEqual(showCallCount, 0, 'onShow not called initially');

    // Manually trigger show
    dialogInstance.show();
    await settled();

    assert.strictEqual(showCallCount, 1, 'onShow called once after show');
    assert.ok(showTarget, 'target element passed to onShow');
    assert.dom(showTarget).hasAttribute('data-test-dialog');
  });

  test('it calls onHide callback when dialog is hidden', async function (assert) {
    let hideCallCount = 0;
    let hideTarget = null;
    let dialogInstance = null;

    this.handleSetup = (dialog) => {
      dialogInstance = dialog;
    };

    this.handleHide = ({ target }) => {
      hideCallCount++;
      hideTarget = target;
    };

    await render(hbs`
      <div {{a11y-dialog onSetup=this.handleSetup onHide=this.handleHide}} data-test-dialog>
        <div role="dialog">
          <button data-a11y-dialog-hide>Close</button>
          <div>Dialog content</div>
        </div>
      </div>
    `);

    // Show then hide
    dialogInstance.show();
    await settled();

    assert.strictEqual(hideCallCount, 0, 'onHide not called after show');

    dialogInstance.hide();
    await settled();

    assert.strictEqual(hideCallCount, 1, 'onHide called once after hide');
    assert.ok(hideTarget, 'target element passed to onHide');
    assert.dom(hideTarget).hasAttribute('data-test-dialog');
  });

  test('it auto-opens dialog when autoOpen is true', async function (assert) {
    let showCallCount = 0;

    this.handleShow = () => {
      showCallCount++;
    };

    await render(hbs`
      <div {{a11y-dialog onShow=this.handleShow autoOpen=true}}>
        <div role="dialog">
          <button data-a11y-dialog-hide>Close</button>
          <div>Dialog content</div>
        </div>
      </div>
    `);

    // render() already calls settled(), so autoOpen should have triggered
    assert.strictEqual(showCallCount, 1, 'dialog was automatically shown');
  });

  test('it does not auto-open when autoOpen is false', async function (assert) {
    let showCallCount = 0;

    this.handleShow = () => {
      showCallCount++;
    };

    await render(hbs`
      <div {{a11y-dialog onShow=this.handleShow autoOpen=false}}>
        <div role="dialog">
          <button data-a11y-dialog-hide>Close</button>
          <div>Dialog content</div>
        </div>
      </div>
    `);

    // render() already calls settled()
    assert.strictEqual(showCallCount, 0, 'dialog was not automatically shown');
  });

  test('it works without onShow callback', async function (assert) {
    let dialogInstance = null;

    this.handleSetup = (dialog) => {
      dialogInstance = dialog;
    };

    await render(hbs`
      <div {{a11y-dialog onSetup=this.handleSetup}}>
        <div role="dialog">
          <button data-a11y-dialog-hide>Close</button>
          <div>Dialog content</div>
        </div>
      </div>
    `);

    // Should not throw an error
    dialogInstance.show();
    await settled();

    assert.ok(true, 'dialog works without onShow callback');
  });

  test('it works without onHide callback', async function (assert) {
    let dialogInstance = null;

    this.handleSetup = (dialog) => {
      dialogInstance = dialog;
    };

    await render(hbs`
      <div {{a11y-dialog onSetup=this.handleSetup}}>
        <div role="dialog">
          <button data-a11y-dialog-hide>Close</button>
          <div>Dialog content</div>
        </div>
      </div>
    `);

    dialogInstance.show();
    await settled();

    // Should not throw an error
    dialogInstance.hide();
    await settled();

    assert.ok(true, 'dialog works without onHide callback');
  });

  test('it works without onSetup callback', async function (assert) {
    await render(hbs`
      <div {{a11y-dialog}}>
        <div role="dialog">
          <button data-a11y-dialog-hide>Close</button>
          <div>Dialog content</div>
        </div>
      </div>
    `);

    assert.ok(true, 'modifier works without onSetup callback');
  });

  test('it cleans up dialog on element destruction', async function (assert) {
    class TestState {
      @tracked showDialog = true;
    }

    const state = new TestState();
    this.state = state;

    let dialogInstance = null;
    let destroyCalled = false;

    this.handleSetup = (dialog) => {
      dialogInstance = dialog;
      // Wrap destroy to detect when it's called
      const originalDestroy = dialog.destroy.bind(dialog);
      dialog.destroy = function () {
        destroyCalled = true;
        return originalDestroy();
      };
    };

    await render(hbs`
      {{#if this.state.showDialog}}
        <div {{a11y-dialog onSetup=this.handleSetup}}>
          <div role="dialog">
            <button data-a11y-dialog-hide>Close</button>
            <div>Dialog content</div>
          </div>
        </div>
      {{/if}}
    `);

    assert.ok(dialogInstance, 'dialog instance was created');
    assert.false(destroyCalled, 'destroy not called yet');

    // Remove the element
    state.showDialog = false;
    await waitUntil(() => destroyCalled, { timeout: 1000 });

    assert.true(destroyCalled, 'destroy was called when element removed');
  });

  test('it handles multiple show/hide cycles', async function (assert) {
    let showCallCount = 0;
    let hideCallCount = 0;
    let dialogInstance = null;

    this.handleSetup = (dialog) => {
      dialogInstance = dialog;
    };

    this.handleShow = () => {
      showCallCount++;
    };

    this.handleHide = () => {
      hideCallCount++;
    };

    await render(hbs`
      <div {{a11y-dialog onSetup=this.handleSetup onShow=this.handleShow onHide=this.handleHide}}>
        <div role="dialog">
          <button data-a11y-dialog-hide>Close</button>
          <div>Dialog content</div>
        </div>
      </div>
    `);

    // First cycle
    dialogInstance.show();
    await settled();
    assert.strictEqual(showCallCount, 1, 'onShow called once');

    dialogInstance.hide();
    await settled();
    assert.strictEqual(hideCallCount, 1, 'onHide called once');

    // Second cycle
    dialogInstance.show();
    await settled();
    assert.strictEqual(showCallCount, 2, 'onShow called twice');

    dialogInstance.hide();
    await settled();
    assert.strictEqual(hideCallCount, 2, 'onHide called twice');

    // Third cycle
    dialogInstance.show();
    await settled();
    assert.strictEqual(showCallCount, 3, 'onShow called three times');

    dialogInstance.hide();
    await settled();
    assert.strictEqual(hideCallCount, 3, 'onHide called three times');
  });

  test('it provides dialog instance immediately in onSetup', async function (assert) {
    let setupCallCount = 0;
    let dialogInSetup = null;

    this.handleSetup = (dialog) => {
      setupCallCount++;
      dialogInSetup = dialog;
    };

    await render(hbs`
      <div {{a11y-dialog onSetup=this.handleSetup}}>
        <div role="dialog">
          <button data-a11y-dialog-hide>Close</button>
          <div>Dialog content</div>
        </div>
      </div>
    `);

    assert.strictEqual(setupCallCount, 1, 'onSetup called once');
    assert.ok(dialogInSetup, 'dialog instance provided in onSetup');
    assert.strictEqual(typeof dialogInSetup.show, 'function', 'dialog is usable in onSetup');
  });

  test('it handles callback changes gracefully', async function (assert) {
    class TestState {
      @tracked showCallback;
    }

    const state = new TestState();
    let firstCallbackCount = 0;
    let secondCallbackCount = 0;
    let dialogInstance = null;

    state.showCallback = () => {
      firstCallbackCount++;
    };

    this.state = state;
    this.handleSetup = (dialog) => {
      dialogInstance = dialog;
    };

    await render(hbs`
      <div {{a11y-dialog onSetup=this.handleSetup onShow=this.state.showCallback}}>
        <div role="dialog">
          <button data-a11y-dialog-hide>Close</button>
          <div>Dialog content</div>
        </div>
      </div>
    `);

    dialogInstance.show();
    await settled();

    assert.strictEqual(firstCallbackCount, 1, 'first callback called');
    assert.strictEqual(secondCallbackCount, 0, 'second callback not called yet');

    dialogInstance.hide();
    await settled();

    // Change the callback
    state.showCallback = () => {
      secondCallbackCount++;
    };

    // Note: The modifier sets up event listeners once, so changing the callback
    // won't affect already-registered listeners. This test documents that behavior.
    dialogInstance.show();
    await settled();

    // The first callback is still bound to the event listener
    assert.strictEqual(
      firstCallbackCount,
      2,
      'first callback still called (listeners set up once)'
    );
    assert.strictEqual(secondCallbackCount, 0, 'second callback not called');
  });

  test('it works with nested dialog structure', async function (assert) {
    let dialogInstance = null;
    let showCalled = false;

    this.handleSetup = (dialog) => {
      dialogInstance = dialog;
    };

    this.handleShow = () => {
      showCalled = true;
    };

    await render(hbs`
      <div {{a11y-dialog onSetup=this.handleSetup onShow=this.handleShow}}>
        <div class="outer-container">
          <div role="dialog" class="inner-dialog">
            <header>
              <button data-a11y-dialog-hide>Close</button>
              <h2>Title</h2>
            </header>
            <div class="content">
              <p>Dialog content</p>
            </div>
            <footer>
              <button>Action</button>
            </footer>
          </div>
        </div>
      </div>
    `);

    assert.ok(dialogInstance, 'dialog created with nested structure');

    dialogInstance.show();
    await settled();

    assert.true(showCalled, 'show event fired with nested structure');
  });
});
