import getComponentFactory from 'consul-ui/utils/get-component-factory';
import { module, test } from 'qunit';

module('Unit | Utility | get component factory');

// Replace this with your real tests.
test('it works', function(assert) {
  let result = getComponentFactory({ lookup: function() {} });
  assert.ok(result);
});
