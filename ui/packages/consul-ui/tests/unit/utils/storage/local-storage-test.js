/**
 * Copyright IBM Corp. 2014, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import localStorage from 'consul-ui/utils/storage/local-storage';
import { module, test } from 'qunit';

module('Unit | Utility | storage/local-storage', function () {
  // Replace this with your real tests.
  const mockStorage = function (obj, encode = (val) => val, decode = (val) => val) {
    return localStorage('test', obj, encode, decode);
  };
  test('getValue returns an empty string if the value is null', function (assert) {
    const expected = '""';
    const storage = mockStorage({
      getItem: function (path) {
        return null;
      },
    });
    const actual = storage.getValue('test');
    assert.strictEqual(actual, expected);
  });
  test('getValue uses the scheme in the path', function (assert) {
    const expected = 'test:test';
    const storage = mockStorage({
      getItem: function (actual) {
        assert.strictEqual(actual, expected);
        return '';
      },
    });
    storage.getValue('test');
  });
  test('setValue uses the scheme in the path', function (assert) {
    const expected = 'test:test';
    const storage = mockStorage({
      setItem: function (actual, value) {
        assert.strictEqual(actual, expected);
        return '';
      },
    });
    storage.setValue('test');
  });
  test('setValue calls removeItem if the value is null', function (assert) {
    const expected = 'test:test';
    const storage = mockStorage({
      removeItem: function (actual) {
        assert.strictEqual(actual, expected);
      },
    });
    storage.setValue('test', null);
  });
  test('all returns an object of kvs under the correct prefix/scheme', function (assert) {
    const storage = mockStorage({
      'tester:a': 'a',
      b: 'b',
      'test:a': 'a',
      'test:b': 'b',
      getItem: function (path) {
        return this[path];
      },
    });
    const expected = {
      a: 'a',
      b: 'b',
    };
    const actual = storage.all();
    assert.deepEqual(actual, expected);
  });
});
