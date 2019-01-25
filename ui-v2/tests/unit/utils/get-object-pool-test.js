import getObjectPool from 'consul-ui/utils/get-object-pool';
import { module, skip } from 'qunit';
import test from 'ember-sinon-qunit/test-support/test';

module('Unit | Utility | get object pool');

skip('Decide what to do if you add 2 objects with the same id');
test('acquire adds objects', function(assert) {
  const actual = [];
  const expected = {
    hi: 'there',
    id: 'hi-there-123',
  };
  const expected2 = {
    hi: 'there',
    id: 'hi-there-456',
  };
  const pool = getObjectPool(function() {}, 10, actual);
  pool.acquire(expected, expected.id);
  assert.deepEqual(actual[0], expected);
  pool.acquire(expected2, expected2.id);
  assert.deepEqual(actual[1], expected2);
});
test('acquire adds objects and returns the id', function(assert) {
  const arr = [];
  const expected = 'hi-there-123';
  const obj = {
    hi: 'there',
    id: expected,
  };
  const pool = getObjectPool(function() {}, 10, arr);
  const actual = pool.acquire(obj, expected);
  assert.equal(actual, expected);
});
test('acquire adds objects, and disposes when there is no room', function(assert) {
  const actual = [];
  const expected = {
    hi: 'there',
    id: 'hi-there-123',
  };
  const expected2 = {
    hi: 'there',
    id: 'hi-there-456',
  };
  const dispose = this.stub()
    .withArgs(expected)
    .returnsArg(0);
  const pool = getObjectPool(dispose, 1, actual);
  pool.acquire(expected, expected.id);
  assert.deepEqual(actual[0], expected);
  pool.acquire(expected2, expected2.id);
  assert.deepEqual(actual[0], expected2);
  assert.ok(dispose.calledOnce);
});
test('it disposes', function(assert) {
  const arr = [];
  const expected = {
    hi: 'there',
    id: 'hi-there-123',
  };
  const expected2 = {
    hi: 'there',
    id: 'hi-there-456',
  };
  const dispose = this.stub().returnsArg(0);
  const pool = getObjectPool(dispose, 2, arr);
  const id = pool.acquire(expected, expected.id);
  assert.deepEqual(arr[0], expected);
  pool.acquire(expected2, expected2.id);
  assert.deepEqual(arr[1], expected2);
  const actual = pool.dispose(id);
  assert.ok(dispose.calledOnce);
  assert.equal(arr.length, 1, 'object was removed from array');
  assert.deepEqual(actual, expected, 'returned object is expected object');
  assert.deepEqual(arr[0], expected2, 'object in the pool is expected object');
});
test('it purges', function(assert) {
  const arr = [];
  const expected = {
    hi: 'there',
    id: 'hi-there-123',
  };
  const expected2 = {
    hi: 'there',
    id: 'hi-there-456',
  };
  const dispose = this.stub().returnsArg(0);
  const pool = getObjectPool(dispose, 2, arr);
  pool.acquire(expected, expected.id);
  assert.deepEqual(arr[0], expected);
  pool.acquire(expected2, expected2.id);
  assert.deepEqual(arr[1], expected2);
  const actual = pool.purge();
  assert.ok(dispose.calledTwice, 'dispose was called on everything');
  assert.equal(arr.length, 0, 'the pool is empty');
  assert.deepEqual(actual[0], expected, 'the first purged object is correct');
  assert.deepEqual(actual[1], expected2, 'the second purged object is correct');
});
