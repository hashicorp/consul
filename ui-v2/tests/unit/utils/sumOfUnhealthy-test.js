import sumOfUnhealthy from 'consul-ui/utils/sumOfUnhealthy';
import { module, test, skip } from 'qunit';

module('Unit | Utility | sum of unhealthy');

test('it returns the correct single count', function(assert) {
  const expected = 1;
  [
    [
      {
        Status: 'critical',
      },
    ],
    [
      {
        Status: 'warning',
      },
    ],
  ].forEach(function(checks) {
    const actual = sumOfUnhealthy(checks);
    assert.equal(actual, expected);
  });
});
test('it returns the correct single count when there are none', function(assert) {
  const expected = 0;
  [
    [
      {
        Status: 'passing',
      },
      {
        Status: 'passing',
      },
      {
        Status: 'passing',
      },
      {
        Status: 'passing',
      },
    ],
    [
      {
        Status: 'passing',
      },
    ],
  ].forEach(function(checks) {
    const actual = sumOfUnhealthy(checks);
    assert.equal(actual, expected);
  });
});
test('it returns the correct multiple count', function(assert) {
  const expected = 3;
  [
    [
      {
        Status: 'critical',
      },
      {
        Status: 'warning',
      },
      {
        Status: 'warning',
      },
      {
        Status: 'passing',
      },
    ],
    [
      {
        Status: 'passing',
      },
      {
        Status: 'critical',
      },
      {
        Status: 'passing',
      },
      {
        Status: 'warning',
      },
      {
        Status: 'warning',
      },
      {
        Status: 'passing',
      },
    ],
  ].forEach(function(checks) {
    const actual = sumOfUnhealthy(checks);
    assert.equal(actual, expected);
  });
});
skip('it works as a factory, passing ember `get` in to create the function');
