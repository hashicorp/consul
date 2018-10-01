import { isManagement } from 'consul-ui/helpers/policy/is-management';
import { module, test } from 'qunit';

module('Unit | Helper | policy/is-management');

test('it returns true if the policy has an id of `00000000-0000-0000-0000-000000000000`', function(assert) {
  const actual = isManagement([{ ID: '00000000-0000-0000-0000-000000000000' }]);
  assert.ok(actual);
});
test("it returns false if the policy has doesn't have an id of `00000000-0000-0000-0000-000000000000`", function(assert) {
  const actual = isManagement([{ ID: '00000000-0000-0000-0000-000000000001' }]);
  assert.ok(!actual);
});
