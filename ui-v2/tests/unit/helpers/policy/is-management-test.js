import { isManagement } from 'consul-ui/helpers/policy/is-management';
import { module, test } from 'qunit';

module('Unit | Helper | policy/is-management');

test('it returns true if the policy is the management policy', function(assert) {
  const actual = isManagement([{ ID: '00000000-0000-0000-0000-000000000001' }]);
  assert.ok(actual);
});
test("it returns false if the policy isn't the management policy", function(assert) {
  const actual = isManagement([{ ID: '00000000-0000-0000-0000-000000000000' }]);
  assert.ok(!actual);
});
