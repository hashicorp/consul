import domEventSourceOpenable from 'consul-ui/utils/dom/event-source/openable';
import { module, test } from 'qunit';
import sinon from 'sinon';

module('Unit | Utility | dom/event-source/openable', function () {
  const createEventSource = function () {
    const EventSource = function (cb) {
      this.readyState = 1;
      this.source = cb;
      this.source.apply(this, arguments);
    };
    const o = EventSource.prototype;
    ['addEventListener', 'removeEventListener', 'dispatchEvent', 'close'].forEach(function (item) {
      o[item] = function () {};
    });
    return EventSource;
  };
  test('it creates an Openable class implementing EventSource', function (assert) {
    const EventSource = createEventSource();
    const OpenableEventSource = domEventSourceOpenable(EventSource);
    assert.ok(OpenableEventSource instanceof Function);
    const source = new OpenableEventSource(function () {});
    assert.ok(source instanceof EventSource);
  });
  test('it reopens the event source when open is called', function (assert) {
    const callable = sinon.stub();
    const EventSource = createEventSource();
    const OpenableEventSource = domEventSourceOpenable(EventSource);
    const source = new OpenableEventSource(callable);
    assert.equal(source.readyState, 1);
    // first automatic EventSource `open`
    assert.ok(callable.calledOnce);
    source.readyState = 3;
    source.open();
    // still only called once as it hasn't completely closed yet
    // therefore is just opened by resetting the readyState
    assert.ok(callable.calledOnce);
    assert.equal(source.readyState, 1);
    // properly close the source
    source.readyState = 2;
    source.open();
    // this time it is opened via a recall of the callable
    assert.ok(callable.calledTwice);
  });
});
