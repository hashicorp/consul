import domEventSourceCallable, { defaultRunner } from 'consul-ui/utils/dom/event-source/callable';
import { module, test } from 'qunit';
import sinon from 'sinon';

module('Unit | Utility | dom/event-source/callable', function () {
  const createEventTarget = function () {
    const EventTarget = function (cb) {};
    const o = EventTarget.prototype;
    ['addEventListener', 'removeEventListener', 'dispatchEvent'].forEach(function (item) {
      o[item] = function () {};
    });
    return EventTarget;
  };
  const createPromise = function () {
    class PromiseMock {
      then(cb) {
        cb();
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
  test('it creates an EventSource class implementing EventTarget', function (assert) {
    const EventTarget = createEventTarget();
    const EventSource = domEventSourceCallable(EventTarget, createPromise());
    assert.ok(EventSource instanceof Function);
    const source = new EventSource();
    assert.ok(source instanceof EventTarget);
  });
  test('the default runner loops and can be closed', function (assert) {
    assert.expect(13); // 10 not closed, 1 to close, the final call count, plus the close event
    let count = 0;
    const isClosed = function () {
      count++;
      assert.ok(true);
      return count === 11;
    };
    const configuration = {};
    const then = sinon.stub().callsArg(0);
    const target = {
      source: function (configuration) {
        return {
          then: then,
        };
      },
      dispatchEvent: sinon.stub(),
    };
    defaultRunner(target, configuration, isClosed);
    assert.equal(then.callCount, 10);
    assert.ok(target.dispatchEvent.calledOnce);
  });
  test('it calls the defaultRunner', function (assert) {
    const Promise = createPromise();
    const EventTarget = createEventTarget();
    const run = sinon.stub();
    const EventSource = domEventSourceCallable(EventTarget, Promise, run);
    const source = new EventSource();
    assert.ok(run.calledOnce);
    assert.equal(source.readyState, 2);
  });
});
