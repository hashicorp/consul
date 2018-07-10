import removeNull from 'consul-ui/utils/remove-null';
import { skip } from 'qunit';
import { module, test } from 'qunit';

module('Unit | Utility | remove null');

test('it removes null valued properties shallowly', function(assert) {
  [
    {
      test: {
        Value: null,
      },
      expected: {},
    },
    {
      test: {
        Key: 'keyname',
        Value: null,
      },
      expected: {
        Key: 'keyname',
      },
    },
    {
      test: {
        Key: 'keyname',
        Value: '',
      },
      expected: {
        Key: 'keyname',
        Value: '',
      },
    },
    {
      test: {
        Key: 'keyname',
        Value: false,
      },
      expected: {
        Key: 'keyname',
        Value: false,
      },
    },
  ].forEach(function(item) {
    const actual = removeNull(item.test);
    assert.deepEqual(actual, item.expected);
  });
});
skip('it removes null valued properties deeply');
