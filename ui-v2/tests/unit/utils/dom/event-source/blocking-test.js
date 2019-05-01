import domEventSourceBlocking, {
  validateCursor,
  create5xxBackoff,
} from 'consul-ui/utils/dom/event-source/blocking';
import { module } from 'qunit';
import test from 'ember-sinon-qunit/test-support/test';

module('Unit | Utility | dom/event-source/blocking');

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
const createPromise = function(resolve = function() {}) {
  class PromiseMock {
    constructor(cb = function() {}) {
      cb(resolve);
    }
    then(cb) {
      setTimeout(() => cb.bind(this)(), 0);
      return this;
    }
    catch(cb) {
      cb({ message: 'error' });
      return this;
    }
  }
  PromiseMock.resolve = function() {
    return new PromiseMock();
  };
  return PromiseMock;
};
test('it creates an BlockingEventSource class implementing EventSource', function(assert) {
  const EventSource = createEventSource();
  const BlockingEventSource = domEventSourceBlocking(EventSource, function() {});
  assert.ok(BlockingEventSource instanceof Function);
  const source = new BlockingEventSource(function() {
    return createPromise().resolve();
  });
  assert.ok(source instanceof EventSource);
});
test("the 5xx backoff continues to throw when it's not a 5xx", function(assert) {
  const backoff = create5xxBackoff();
  [
    undefined,
    null,
    new Error(),
    { errors: [] },
    { errors: [{ status: '0' }] },
    { errors: [{ status: 501 }] },
    { errors: [{ status: '401' }] },
    { errors: [{ status: '500' }] },
    { errors: [{ status: '5' }] },
    { errors: [{ status: '50' }] },
    { errors: [{ status: '5000' }] },
    { errors: [{ status: '5050' }] },
  ].forEach(function(item) {
    assert.throws(function() {
      backoff(item);
    });
  });
});
test('the 5xx backoff returns a resolve promise on a 5xx (apart from 500)', function(assert) {
  [
    { errors: [{ status: '501' }] },
    { errors: [{ status: '503' }] },
    { errors: [{ status: '504' }] },
    { errors: [{ status: '524' }] },
  ].forEach(item => {
    const timeout = this.stub().callsArg(0);
    const resolve = this.stub().withArgs(item);
    const Promise = createPromise(resolve);
    const backoff = create5xxBackoff(undefined, Promise, timeout);
    const promise = backoff(item);
    assert.ok(promise instanceof Promise, 'a promise was returned');
    assert.ok(resolve.calledOnce, 'the promise was resolved with the correct arguments');
    assert.ok(timeout.calledOnce, 'timeout was called once');
  });
});
test("the cursor validation always returns undefined if the cursor can't be parsed to an integer", function(assert) {
  ['null', null, '', undefined].forEach(item => {
    const actual = validateCursor(item);
    assert.equal(actual, undefined);
  });
});
test('the cursor validation always returns a cursor greater than zero', function(assert) {
  [
    {
      cursor: 0,
      expected: 1,
    },
    {
      cursor: -10,
      expected: 1,
    },
    {
      cursor: -1,
      expected: 1,
    },
    {
      cursor: -1000,
      expected: 1,
    },
    {
      cursor: 10,
      expected: 10,
    },
  ].forEach(item => {
    const actual = validateCursor(item.cursor);
    assert.equal(actual, item.expected, 'cursor is greater than zero');
  });
});
test('the cursor validation resets to 1 if its less than the previous cursor', function(assert) {
  [
    {
      previous: 100,
      cursor: 99,
      expected: 1,
    },
    {
      previous: 100,
      cursor: -10,
      expected: 1,
    },
    {
      previous: 100,
      cursor: 0,
      expected: 1,
    },
    {
      previous: 100,
      cursor: 101,
      expected: 101,
    },
  ].forEach(item => {
    const actual = validateCursor(item.cursor, item.previous);
    assert.equal(actual, item.expected);
  });
});
