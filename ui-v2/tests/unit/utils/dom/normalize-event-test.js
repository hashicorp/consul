import domNormalizeEvent from 'consul-ui/utils/dom/normalize-event';
import { module, test } from 'qunit';

module('Unit | Utility | dom/normalize event', function() {
  test('it returns the same object if target is defined', function(assert) {
    const expected = { target: true };
    const actual = domNormalizeEvent(expected, 'value');
    assert.deepEqual(actual, expected);
  });
  test('it returns an event-like object if target is undefined', function(assert) {
    const expected = { target: { name: 'name', value: 'value' } };
    const actual = domNormalizeEvent('name', 'value');
    assert.deepEqual(actual, expected);
  });
});
