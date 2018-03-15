import { module } from 'ember-qunit';
import test from 'ember-sinon-qunit/test-support/test';
import ascend from 'consul-ui/utils/ascend';
module('Unit | Utils | ascend', {});

test('it returns a parent path (ascension of 1)', function(assert) {
  const expected = '/quite/a/deep/path/for/';
  const actual = ascend(expected + 'parent', 1);
  assert.equal(actual, expected);
});
test('it returns a grand parent path (ascension of 2)', function(assert) {
  const expected = 'quite/a/deep/path/for/';
  const actual = ascend(expected + 'grand/parent', 2);
  assert.equal(actual, expected);
});
test('ascending past root returns ""', function(assert) {
  const expected = '';
  const actual = ascend('/short', 2);
  assert.equal(actual, expected);
});
