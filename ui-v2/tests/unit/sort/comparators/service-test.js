import comparatorFactory from 'consul-ui/sort/comparators/service';
import { module, test } from 'qunit';

module('Unit | Sort | Comparator | service', function() {
  const comparator = comparatorFactory();
  test('Passing anything but Status: just returns what you gave it', function(assert) {
    const expected = 'Name:asc';
    const actual = comparator(expected);
    assert.equal(actual, expected);
  });
  test('items are sorted by a fake Status which uses Checks{Passing,Warning,Critical}', function(assert) {
    const items = [
      {
        ChecksPassing: 1,
        ChecksWarning: 1,
        ChecksCritical: 1,
      },
      {
        ChecksPassing: 1,
        ChecksWarning: 1,
        ChecksCritical: 2,
      },
      {
        ChecksPassing: 1,
        ChecksWarning: 1,
        ChecksCritical: 3,
      },
    ];
    const comp = comparator('Status:asc');
    assert.equal(typeof comp, 'function');

    const expected = [
      {
        ChecksPassing: 1,
        ChecksWarning: 1,
        ChecksCritical: 3,
      },
      {
        ChecksPassing: 1,
        ChecksWarning: 1,
        ChecksCritical: 2,
      },
      {
        ChecksPassing: 1,
        ChecksWarning: 1,
        ChecksCritical: 1,
      },
    ];
    let actual = items.sort(comp);
    assert.deepEqual(actual, expected);

    expected.reverse();
    actual = items.sort(comparator('Status:desc'));
    assert.deepEqual(actual, expected);
  });
});
