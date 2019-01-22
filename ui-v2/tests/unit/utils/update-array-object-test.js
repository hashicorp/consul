import updateArrayObject from 'consul-ui/utils/update-array-object';
import { module, test } from 'qunit';

module('Unit | Utility | update array object');

// Replace this with your real tests.
test('it works', function(assert) {
  const expected = {
    data: {
      id: '2',
      name: 'expected',
    },
  };
  const arr = [
    {
      data: {
        id: '1',
        name: 'name',
      },
    },
    {
      data: {
        id: '2',
        name: '-',
      },
    },
  ];
  const actual = updateArrayObject(arr, expected, 'id');
  assert.ok(actual, expected);
});
