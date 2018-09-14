import { callIfType } from 'consul-ui/helpers/format-number';
import { module, test } from 'qunit';

module('Unit | helper | format-number.callIfType');
const formatNumber = callIfType('number')(function() {
  return true;
});
test('it returns the argument passed if not a number', function(assert) {
  assert.equal(formatNumber(['1,000']), '1,000');
});
test("it returns the argument to locale string if it's a number", function(assert) {
  assert.ok(formatNumber([1000]));
});
