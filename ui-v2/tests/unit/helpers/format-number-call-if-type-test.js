import { callIfType } from 'consul-ui/helpers/format-number';
import { module, test } from 'qunit';

module('Unit | helper | format-number.callIfType');
const formatNumber = callIfType('number')(function() {
  return true;
});
/*
 * This util is meant for usage with helpers only, therefore would usually be called with
 * {{ format-number "1,000" }}
 * The first argument in the helper here is eventually converted to the first element
 * of an array by ember (the typical helper function signature)
 * So below, test names refer to 'arguments' to mean 'helper arguments'
 * i.e. {{helper-name "argument-one", "argument-two"}}
*/
test('it returns the helper argument passed if not a number', function(assert) {
  assert.equal(formatNumber(['1,000']), '1,000');
});
test("it returns the helper argument to locale string if it's a number", function(assert) {
  assert.ok(formatNumber([1000]));
});
