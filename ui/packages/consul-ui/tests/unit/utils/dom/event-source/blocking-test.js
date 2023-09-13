/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import domEventSourceBlocking, {
  validateCursor,
  createErrorBackoff,
} from 'consul-ui/utils/dom/event-source/blocking';
import { module, test } from 'qunit';
import sinon from 'sinon';

module('Unit | Utility | dom/event-source/blocking', function () {
  const createEventSource = function () {
    const EventSource = function (cb) {
      this.readyState = 1;
      this.source = cb;
    };
    const o = EventSource.prototype;
    ['addEventListener', 'removeEventListener', 'dispatchEvent', 'close'].forEach(function (item) {
      o[item] = function () {};
    });
    return EventSource;
  };
  const createPromise = function (resolve = function () {}) {
    class PromiseMock {
      constructor(cb = function () {}) {
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
    PromiseMock.resolve = function () {
      return new PromiseMock();
    };
    return PromiseMock;
  };
  test('it creates an BlockingEventSource class implementing EventSource', function (assert) {
    const EventSource = createEventSource();
    const BlockingEventSource = domEventSourceBlocking(EventSource, function () {});
    assert.ok(BlockingEventSource instanceof Function);
    const source = new BlockingEventSource(function () {
      return createPromise().resolve();
    });
    assert.ok(source instanceof EventSource);
  });
  test("the 5xx backoff continues to throw when it's not a 5xx", function (assert) {
    assert.expect(11);

    const backoff = createErrorBackoff();
    [
      undefined,
      null,
      new Error(),
      { statusCode: 404 },
      { errors: [] },
      { errors: [{ status: '401' }] },
      { errors: [{ status: '500' }] },
      { errors: [{ status: '5' }] },
      { errors: [{ status: '50' }] },
      { errors: [{ status: '5000' }] },
      { errors: [{ status: '5050' }] },
    ].forEach(function (item) {
      assert.throws(function () {
        backoff(item);
      });
    });
  });
  test('the 5xx backoff returns a resolve promise on a 5xx (apart from 500)', function (assert) {
    assert.expect(18);

    [
      { statusCode: 501 },
      { errors: [{ status: 501 }] },
      { errors: [{ status: '501' }] },
      { errors: [{ status: '503' }] },
      { errors: [{ status: '504' }] },
      { errors: [{ status: '524' }] },
    ].forEach((item) => {
      const timeout = sinon.stub().callsArg(0);
      const resolve = sinon.stub().withArgs(item);
      const Promise = createPromise(resolve);
      const backoff = createErrorBackoff(undefined, Promise, timeout);
      const promise = backoff(item);
      assert.ok(promise instanceof Promise, 'a promise was returned');
      assert.ok(resolve.calledOnce, 'the promise was resolved with the correct arguments');
      assert.ok(timeout.calledOnce, 'timeout was called once');
    });
  });
  test("the cursor validation always returns undefined if the cursor can't be parsed to an integer", function (assert) {
    assert.expect(4);

    ['null', null, '', undefined].forEach((item) => {
      const actual = validateCursor(item);
      assert.equal(actual, undefined);
    });
  });
  test('the cursor validation always returns a cursor greater than zero', function (assert) {
    assert.expect(5);

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
    ].forEach((item) => {
      const actual = validateCursor(item.cursor);
      assert.equal(actual, item.expected, 'cursor is greater than zero');
    });
  });
  test('the cursor validation resets to 1 if its less than the previous cursor', function (assert) {
    assert.expect(4);

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
    ].forEach((item) => {
      const actual = validateCursor(item.cursor, item.previous);
      assert.equal(actual, item.expected);
    });
  });
});
