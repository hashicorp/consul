import { module } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { clearRender, render, waitUntil } from '@ember/test-helpers';
import hbs from 'htmlbars-inline-precompile';

import test from 'ember-sinon-qunit/test-support/test';
import Service from '@ember/service';

import { BlockingEventSource as RealEventSource } from 'consul-ui/utils/dom/event-source';

const createFakeBlockingEventSource = function() {
  const EventSource = function(cb) {
    this.readyState = 1;
    this.source = cb;
  };
  const o = EventSource.prototype;
  [
    'addEventListener',
    'removeEventListener',
    'dispatchEvent',
    'close',
    'open',
    'getCurrentEvent',
  ].forEach(function(item) {
    o[item] = function() {};
  });
  return EventSource;
};
const BlockingEventSource = createFakeBlockingEventSource();
module('Integration | Component | data-source', function(hooks) {
  setupRenderingTest(hooks);

  hooks.beforeEach(function() {
    this.actions = {};
    this.send = (actionName, ...args) => this.actions[actionName].apply(this, args);
  });
  test('open and closed are called correctly when the src is changed', async function(assert) {
    // Set any properties with this.set('myProperty', 'value');
    // Handle any actions with this.set('myAction', function(val) { ... });
    assert.expect(9);
    const close = this.stub();
    const open = this.stub();
    const addEventListener = this.stub();
    const removeEventListener = this.stub();
    let count = 0;
    const blockingStub = Service.extend({
      open: function(uri, obj) {
        open(uri);
        const source = new BlockingEventSource();
        source.getCurrentEvent = function() {
          return { data: uri };
        };
        source.addEventListener = addEventListener;
        source.removeEventListener = removeEventListener;
        return source;
      },
      close: close,
    });
    this.owner.register('service:blocking', blockingStub);
    this.actions.change = data => {
      count++;
      switch (count) {
        case 1:
          assert.equal(data, 'a', 'change was called first with "a"');
          this.set('src', 'b');
          break;
        case 2:
          assert.equal(data, 'b', 'change was called second with "b"');
          break;
      }
    };

    this.set('src', 'a');
    await render(hbs`{{data-source src=src onchange=(action 'change' value="data")}}`);
    assert.equal(this.element.textContent.trim(), '');
    await waitUntil(() => {
      return close.calledTwice;
    });
    assert.ok(open.calledTwice, 'open is called when src is set');
    assert.ok(close.calledTwice, 'close is called as open is called');
    await clearRender();
    await waitUntil(() => {
      return close.calledThrice;
    });
    assert.ok(open.calledTwice, 'open is _still_ only called when src is set');
    assert.ok(close.calledThrice, 'close is called an extra time as the component is destroyed');
    assert.equal(addEventListener.callCount, 4, 'all event listeners were added');
    assert.equal(removeEventListener.callCount, 4, 'all event listeners were removed');
  });
  test('error actions are triggered when errors are dispatched', async function(assert) {
    const source = new RealEventSource();
    const error = this.stub();
    const close = this.stub();
    const blockingStub = Service.extend({
      open: function(uri, obj) {
        source.getCurrentEvent = function() {
          return {};
        };
        return source;
      },
      close: close,
    });
    this.owner.register('service:blocking', blockingStub);
    this.actions.change = data => {
      source.dispatchEvent({ type: 'error', error: {} });
    };
    this.actions.error = error;
    await render(
      hbs`{{data-source src="" onchange=(action 'change' value="data") onerror=(action 'error' value="error")}}`
    );
    await waitUntil(() => {
      return error.calledOnce;
    });
    assert.ok(error.calledOnce, 'error action was called');
    assert.ok(close.calledOnce, 'close was called before the open');
    await clearRender();
    assert.ok(close.calledTwice, 'close was also called when the component is destroyed');
  });
});
