import callableType from 'consul-ui/utils/callable-type';
import { module, test } from 'qunit';

module('Unit | Utility | callable type', function() {
  test('returns a function returning the string', function(assert) {
    const expected = 'hi';
    const actual = callableType(expected)();
    assert.equal(actual, expected);
  });
  test('returns the same function if you pass it a function', function(assert) {
    const expected = 'hi';
    const actual = callableType(function() {
      return 'hi';
    })();
    assert.equal(actual, expected);
  });
});
