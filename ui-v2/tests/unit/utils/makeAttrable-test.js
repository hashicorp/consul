import { module } from 'ember-qunit';
import test from 'ember-sinon-qunit/test-support/test';
import makeAttrable from 'consul-ui/utils/makeAttrable';
module('Unit | Utils | makeAttrable', {});

test('it adds a `attr` method, which returns the value of the property', function(assert) {
  const obj = {
    prop: true,
  };
  const actual = makeAttrable(obj);
  assert.equal(typeof actual.attr, 'function');
  assert.ok(actual.attr('prop'));
});
