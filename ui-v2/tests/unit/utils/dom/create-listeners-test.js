import createListeners from 'consul-ui/utils/dom/create-listeners';
import { module } from 'ember-qunit';
import test from 'ember-sinon-qunit/test-support/test';

module('Unit | Utility | dom/create listeners');

test('it has add and remove methods', function(assert) {
  const listeners = createListeners();
  assert.ok(typeof listeners.add === 'function');
  assert.ok(typeof listeners.remove === 'function');
});
test('add returns an remove function', function(assert) {
  const listeners = createListeners();
  const remove = listeners.add({
    addEventListener: function() {},
  });
  assert.ok(typeof remove === 'function');
});
test('remove returns the listeners', function(assert) {
  const expected = [function() {}];
  const listeners = createListeners(expected);
  const actual = listeners.remove();
  assert.deepEqual(actual, expected);
  assert.equal(expected.length, 0);
});
test('remove calls the remove functions', function(assert) {
  const expected = this.stub();
  const arr = [expected];
  const listeners = createListeners(arr);
  listeners.remove();
  assert.ok(expected.calledOnce);
  assert.equal(arr.length, 0);
});
test('listeners are added on add', function(assert) {
  const listeners = createListeners();
  const stub = this.stub();
  const target = {
    addEventListener: stub,
  };
  const name = 'test';
  const handler = function(e) {};
  listeners.add(target, name, handler);
  assert.ok(stub.calledOnce);
  assert.ok(stub.calledWith(name, handler));
});
test('listeners are removed on remove', function(assert) {
  const listeners = createListeners();
  const stub = this.stub();
  const target = {
    addEventListener: function() {},
    removeEventListener: stub,
  };
  const name = 'test';
  const handler = function(e) {};
  const remove = listeners.add(target, name, handler);
  remove();
  assert.ok(stub.calledOnce);
  assert.ok(stub.calledWith(name, handler));
});
