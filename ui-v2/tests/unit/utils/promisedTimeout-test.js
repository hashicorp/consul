import { module, skip } from 'ember-qunit';
import test from 'ember-sinon-qunit/test-support/test';
import promisedTimeout from 'consul-ui/utils/promisedTimeout';
module('Unit | Utils | promisedTimeout', {});

test('it calls setTimeout with the correct milliseconds', function(assert) {
  const expected = 1000;
  const P = function(cb) {
    cb(function(milliseconds) {
      assert.equal(milliseconds, expected);
    });
  };
  const setTimeoutDouble = function(cb, milliseconds) {
    assert.equal(milliseconds, expected);
    cb();
    return 1;
  };
  const timeout = promisedTimeout(P, setTimeoutDouble);
  timeout(expected, function() {});
});
skip('it still clears the interval if there is no callback');
