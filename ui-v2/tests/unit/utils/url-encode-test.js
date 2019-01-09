import { module } from 'ember-qunit';
import test from 'ember-sinon-qunit/test-support/test';
import urlEncode from 'consul-ui/utils/url-encode';

module('Unit | Utility | url encode');

test('it calls the encode on an array of strings', function(assert) {
  const encoder = this.stub().returnsArg(0);
  const encode = urlEncode(encoder);
  const expected = ['one', 'two', 'three', 'four'];
  const actual = encode(expected);
  expected.forEach(function(item) {
    assert.ok(encoder.calledWith(item));
  });
  assert.equal(encoder.callCount, expected.length);
  assert.deepEqual(actual, expected);
});
test('it calls the encode on an array of strings and arrays, arrays get joined by slashes', function(assert) {
  const encoder = this.stub().returnsArg(0);
  const encode = urlEncode(encoder);
  const expected = ['one', 'a/b/c/d/e', 'three', 'four'];
  const data = ['one', ['a', 'b', ['c', 'd', 'e']], 'three', 'four'];
  const actual = encode(data);
  data.forEach(function recur(item) {
    if (Array.isArray(item)) {
      item.forEach(recur);
    } else {
      assert.ok(encoder.calledWith(item));
    }
  });
  assert.equal(encoder.callCount, 8);
  assert.deepEqual(actual, expected);
});
