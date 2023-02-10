import createListeners from 'consul-ui/utils/dom/create-listeners';
import { module, test } from 'qunit';
import sinon from 'sinon';

module('Unit | Utility | dom/create listeners', function () {
  test('it has add and remove methods', function (assert) {
    const listeners = createListeners();
    assert.strictEqual(typeof listeners.add, 'function');
    assert.strictEqual(typeof listeners.remove, 'function');
  });
  test('add returns a remove function', function (assert) {
    const listeners = createListeners();
    const remove = listeners.add(
      {
        addEventListener: function () {},
      },
      'click'
    );
    assert.strictEqual(typeof remove, 'function');
  });
  test('remove returns an array of removed handlers (the return of a saved remove)', function (assert) {
    // just use true here to prove that it's what gets returned
    const expected = true;
    const handlers = [
      function () {
        return expected;
      },
    ];
    const listeners = createListeners(handlers);
    const actual = listeners.remove();
    assert.deepEqual(actual, [expected]);
    // handlers should now be empty
    assert.equal(handlers.length, 0);
  });
  test('remove calls the remove functions', function (assert) {
    const expected = sinon.stub();
    const arr = [expected];
    const listeners = createListeners(arr);
    listeners.remove();
    assert.ok(expected.calledOnce);
    assert.equal(arr.length, 0);
  });
  test('listeners are added on add', function (assert) {
    const listeners = createListeners();
    const stub = sinon.stub();
    const target = {
      addEventListener: stub,
    };
    const name = 'test';
    const handler = function (e) {};
    listeners.add(target, name, handler);
    assert.ok(stub.calledOnce);
    assert.ok(stub.calledWith(name, handler));
  });
  test('listeners as objects are added on add and removed on remove', function (assert) {
    const listeners = createListeners();
    const addStub = sinon.stub();
    const removeStub = sinon.stub();
    const target = {
      addEventListener: addStub,
      removeEventListener: removeStub,
    };
    const handler = function (e) {};
    const remove = listeners.add(target, {
      message: handler,
      error: handler,
    });
    assert.ok(addStub.calledTwice);
    assert.ok(addStub.calledWith('message', handler));
    assert.ok(addStub.calledWith('error', handler));

    remove();

    assert.ok(removeStub.calledTwice);
    assert.ok(removeStub.calledWith('message', handler));
    assert.ok(removeStub.calledWith('error', handler));
  });
  test('listeners are removed on remove', function (assert) {
    const listeners = createListeners();
    const stub = sinon.stub();
    const target = {
      addEventListener: function () {},
      removeEventListener: stub,
    };
    const name = 'test';
    const handler = function (e) {};
    const remove = listeners.add(target, name, handler);
    remove();
    assert.ok(stub.calledOnce);
    assert.ok(stub.calledWith(name, handler));
  });
  test('listeners as functions are removed on remove', function (assert) {
    const listeners = createListeners();
    const stub = sinon.stub();
    const remove = listeners.add(stub);
    remove();
    assert.ok(stub.calledOnce);
  });
  test('listeners as other listeners are removed on remove', function (assert) {
    const listeners = createListeners();
    const listeners2 = createListeners();
    const stub = sinon.stub();
    listeners2.add(stub);
    const remove = listeners.add(listeners2);
    remove();
    assert.ok(stub.calledOnce);
  });
  test('listeners as functions of other listeners are removed on remove', function (assert) {
    const listeners = createListeners();
    const listeners2 = createListeners();
    const stub = sinon.stub();
    const remove = listeners.add(listeners2.add(stub));
    remove();
    assert.ok(stub.calledOnce);
  });
  test('remove returns an array containing the original handler', function (assert) {
    const listeners = createListeners();
    const target = {
      addEventListener: function () {},
      removeEventListener: function () {},
    };
    const name = 'test';
    const expected = sinon.stub();
    const remove = listeners.add(target, name, expected);
    const actual = remove();
    actual[0]();
    assert.ok(expected.calledOnce);
  });
});
