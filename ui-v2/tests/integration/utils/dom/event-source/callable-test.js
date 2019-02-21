import domEventSourceCallable from 'consul-ui/utils/dom/event-source/callable';
import EventTarget from 'consul-ui/utils/dom/event-target/rsvp';
import { Promise } from 'rsvp';

import { module } from 'qunit';
import { setupTest } from 'ember-qunit';
import test from 'ember-sinon-qunit/test-support/test';

module('Integration | Utility | dom/event-source/callable', function(hooks) {
  setupTest(hooks);
  test('it dispatches messages', function(assert) {
    assert.expect(1);
    const EventSource = domEventSourceCallable(EventTarget);
    const listener = this.stub();
    const source = new EventSource(
      function(configuration) {
        return new Promise(resolve => {
          setTimeout(() => {
            this.dispatchEvent({
              type: 'message',
              data: null,
            });
            resolve();
          }, configuration.milliseconds);
        });
      },
      {
        milliseconds: 100,
      }
    );
    source.addEventListener('message', function() {
      listener();
    });
    return new Promise(function(resolve) {
      setTimeout(function() {
        source.close();
        assert.equal(listener.callCount, 5);
        resolve();
      }, 550);
    });
  });
  test('it dispatches a single open event and closes when called with no callable', function(assert) {
    assert.expect(4);
    const EventSource = domEventSourceCallable(EventTarget);
    const listener = this.stub();
    const source = new EventSource();
    source.addEventListener('open', function(e) {
      assert.deepEqual(e.target, this);
      assert.equal(e.target.readyState, 1);
      listener();
    });
    return Promise.resolve().then(function() {
      assert.ok(listener.calledOnce);
      assert.equal(source.readyState, 2);
    });
  });
  test('it dispatches a single open event, and calls the specified callable that can dispatch an event', function(assert) {
    assert.expect(1);
    const EventSource = domEventSourceCallable(EventTarget);
    const listener = this.stub();
    const source = new EventSource(function() {
      return new Promise(resolve => {
        setTimeout(() => {
          this.dispatchEvent({
            type: 'message',
            data: {},
          });
          this.close();
        }, 190);
      });
    });
    source.addEventListener('open', function() {
      // open is called first
      listener();
    });
    return new Promise(function(resolve) {
      source.addEventListener('message', function() {
        // message is called second
        assert.ok(listener.calledOnce);
        resolve();
      });
    });
  });
});
