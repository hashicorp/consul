import hasStatus from 'consul-ui/utils/hasStatus';
import { module, test, skip } from 'qunit';

module('Unit | Utility | has status');

const checks = {
  filterBy: function(prop, value) {
    return { length: 0 };
  },
};
test('it returns true when passing an empty string (therefore "all")', function(assert) {
  assert.ok(hasStatus(checks, ''));
});
test('it returns false when passing an actual status', function(assert) {
  ['passing', 'critical', 'warning'].forEach(function(item) {
    assert.ok(!hasStatus(checks, item), `, with ${item}`);
  });
});
skip('it works as a factory, passing ember `get` in to create the function');
