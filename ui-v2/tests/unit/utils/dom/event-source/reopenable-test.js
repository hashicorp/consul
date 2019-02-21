import domEventSourceReopenable from 'consul-ui/utils/dom/event-source/reopenable';
import { module } from 'qunit';
import test from 'ember-sinon-qunit/test-support/test';

module('Unit | Utility | dom/event-source/reopenable');

const createEventSource = function() {
  return class {
    constructor(cb) {
      this.readyState = 1;
      this.source = cb;
      this.source.apply(this, arguments);
    }
    addEventListener() {}
    removeEventListener() {}
    dispatchEvent() {}
    close() {}
  };
};
test('it creates an Reopenable class implementing EventSource', function(assert) {
  const EventSource = createEventSource();
  const ReopenableEventSource = domEventSourceReopenable(EventSource);
  assert.ok(ReopenableEventSource instanceof Function);
  const source = new ReopenableEventSource(function() {});
  assert.ok(source instanceof EventSource);
});
test('it reopens the event source when reopen is called', function(assert) {
  const callable = this.stub();
  const EventSource = createEventSource();
  const ReopenableEventSource = domEventSourceReopenable(EventSource);
  const source = new ReopenableEventSource(callable);
  assert.equal(source.readyState, 1);
  // first automatic EventSource `open`
  assert.ok(callable.calledOnce);
  source.readyState = 3;
  source.reopen();
  // still only called once as it hasn't completely closed yet
  // therefore is just opened by resetting the readyState
  assert.ok(callable.calledOnce);
  assert.equal(source.readyState, 1);
  // properly close the source
  source.readyState = 2;
  source.reopen();
  // this time it is reopened via a recall of the callable
  assert.ok(callable.calledTwice);
});
