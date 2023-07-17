import { datacenters } from 'consul-ui/helpers/policy/datacenters';
import { module, test } from 'qunit';

module('Unit | Helper | policy/datacenters', function () {
  test('it returns "All" if you pass a policy with no Datacenters property', function (assert) {
    const expected = ['All'];
    const actual = datacenters([{}]);
    assert.deepEqual(actual, expected);
  });
  test('it returns "All" if you pass a policy with an empty Array as its Datacenters property', function (assert) {
    const expected = ['All'];
    const actual = datacenters([{ Datacenters: [] }]);
    assert.deepEqual(actual, expected);
  });
  test('it returns "All" if you pass a policy with anything but an array', function (assert) {
    // we know this uses isArray so lets just test with null as thats slightly likely
    const expected = ['All'];
    const actual = datacenters([{ Datacenters: null }]);
    assert.deepEqual(actual, expected);
  });
  test('it returns the Datacenters if you pass a policy with correctly set Datacenters', function (assert) {
    const expected = ['dc-1', 'dc-2'];
    const actual = datacenters([{ Datacenters: ['dc-1', 'dc-2'] }]);
    assert.deepEqual(actual, expected);
  });
});
