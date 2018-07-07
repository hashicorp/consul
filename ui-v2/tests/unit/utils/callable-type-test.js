import callableType from 'consul-ui/utils/callable-type';
import { module, test } from 'qunit';
// TODO: Sure there's a better name for this
module('Unit | Utility | callable type');

test('returns a function returning the string', function(assert) {
  const expected = 'hi';
  const actual = callableType(expected)();
  assert.equal(actual, expected);
});
