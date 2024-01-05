/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import domEventSourceCache from 'consul-ui/utils/dom/event-source/cache';
import { module, test } from 'qunit';
import sinon from 'sinon';

module('Unit | Utility | dom/event-source/cache', function () {
  const createEventSource = function () {
    return class {
      constructor(cb) {
        this.source = cb;
        this.source.apply(this, arguments);
      }
      addEventListener() {}
      removeEventListener() {}
      dispatchEvent() {}
      close() {}
    };
  };
  const createPromise = function (
    resolve = (result) => result,
    reject = (result = { message: 'error' }) => result
  ) {
    class PromiseMock {
      constructor(cb = function () {}) {
        cb(resolve);
      }
      then(cb) {
        setTimeout(() => cb.bind(this)(resolve()), 0);
        return this;
      }
      catch(cb) {
        setTimeout(() => cb.bind(this)(reject()), 0);
        return this;
      }
    }
    PromiseMock.resolve = function (result) {
      return new PromiseMock(function (resolve) {
        resolve(result);
      });
    };
    PromiseMock.reject = function () {
      return new PromiseMock();
    };
    return PromiseMock;
  };
  test('it returns a function', function (assert) {
    const EventSource = createEventSource();
    const Promise = createPromise();

    const getCache = domEventSourceCache(function () {}, EventSource, Promise);
    assert.strictEqual(typeof getCache, 'function');
  });
  test('getCache returns a function', function (assert) {
    const EventSource = createEventSource();
    const Promise = createPromise();

    const getCache = domEventSourceCache(function () {}, EventSource, Promise);
    const obj = {};
    const cache = getCache(obj);
    assert.strictEqual(typeof cache, 'function');
  });
  test('cache creates the default EventSource and keeps it open when there is a cursor', function (assert) {
    const EventSource = createEventSource();
    const stub = {
      configuration: { cursor: 1 },
    };
    const Promise = createPromise(function () {
      return stub;
    });
    const source = sinon.stub().returns(Promise.resolve());
    const cb = sinon.stub();
    const getCache = domEventSourceCache(source, EventSource, Promise);
    const obj = {};
    const cache = getCache(obj);
    const promisedEventSource = cache(cb, {
      key: 'key',
      settings: {
        enabled: true,
      },
    });
    assert.ok(source.calledOnce, 'promisifying source called once');
    assert.ok(promisedEventSource instanceof Promise, 'source returns a Promise');
    const retrievedEventSource = cache(cb, {
      key: 'key',
      settings: {
        enabled: true,
      },
    });
    assert.deepEqual(promisedEventSource, retrievedEventSource);
    assert.ok(source.calledTwice, 'promisifying source called once');
    assert.ok(retrievedEventSource instanceof Promise, 'source returns a Promise');
  });
  test('cache creates the default EventSource and keeps it open when there is a cursor 2', function (assert) {
    assert.expect(4);

    const EventSource = createEventSource();
    const stub = {
      close: sinon.stub(),
      configuration: { cursor: 1 },
    };
    const Promise = createPromise(function () {
      return stub;
    });
    const source = sinon.stub().returns(Promise.resolve());
    const cb = sinon.stub();
    const getCache = domEventSourceCache(source, EventSource, Promise);
    const obj = {};
    const cache = getCache(obj);
    const promisedEventSource = cache(cb, {
      key: 0,
      settings: {
        enabled: true,
      },
    });
    assert.ok(source.calledOnce, 'promisifying source called once');
    assert.ok(cb.calledOnce, 'callable event source callable called once');
    assert.ok(promisedEventSource instanceof Promise, 'source returns a Promise');
    // >>
    return promisedEventSource.then(function () {
      assert.notOk(stub.close.called, "close wasn't called");
    });
  });
  test("cache creates the default EventSource and closes it when there isn't a cursor", function (assert) {
    assert.expect(4);

    const EventSource = createEventSource();
    const stub = {
      close: sinon.stub(),
      configuration: {},
    };
    const Promise = createPromise(function () {
      return stub;
    });
    const source = sinon.stub().returns(Promise.resolve());
    const cb = sinon.stub();
    const getCache = domEventSourceCache(source, EventSource, Promise);
    const obj = {};
    const cache = getCache(obj);
    const promisedEventSource = cache(cb, {
      key: 0,
    });
    assert.ok(source.calledOnce, 'promisifying source called once');
    assert.ok(cb.calledOnce, 'callable event source callable called once');
    assert.ok(promisedEventSource instanceof Promise, 'source returns a Promise');
    // >>
    return promisedEventSource.then(function () {
      assert.ok(stub.close.calledOnce, 'close was called');
    });
  });
});
