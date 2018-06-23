import { module } from 'ember-qunit';
import test from 'ember-sinon-qunit/test-support/test';
import confirm from 'consul-ui/utils/confirm';
module('Unit | Utils | confirm', {});

test('it resolves the result of the confirmation', function(assert) {
  const expected = 'message';
  // split this off into separate testing
  const confirmation = function(actual) {
    assert.equal(actual, expected);
    return true;
  };
  return confirm(expected, confirmation).then(function(res) {
    assert.ok(res);
  });
});
