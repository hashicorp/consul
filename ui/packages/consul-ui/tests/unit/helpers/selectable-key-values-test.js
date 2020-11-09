import { selectableKeyValues } from 'consul-ui/helpers/selectable-key-values';
import { module, test } from 'qunit';

module('Unit | Helper | selectable-key-values', function() {
  test('it turns arrays into key values and selects the first item by default', function(assert) {
    const actual = selectableKeyValues([['key-1', 'value-1'], ['key-2', 'value-2']]);
    assert.equal(actual.items.length, 2);
    assert.deepEqual(actual.selected, { key: 'key-1', value: 'value-1' });
  });
  test('it turns arrays into key values and selects the defined key', function(assert) {
    const actual = selectableKeyValues([['key-1', 'value-1'], ['key-2', 'value-2']], {
      selected: 'key-2',
    });
    assert.equal(actual.items.length, 2);
    assert.deepEqual(actual.selected, { key: 'key-2', value: 'value-2' });
  });
  test('it turns arrays into key values and selects the defined index', function(assert) {
    const actual = selectableKeyValues([['key-1', 'value-1'], ['key-2', 'value-2']], {
      selected: 1,
    });
    assert.equal(actual.items.length, 2);
    assert.deepEqual(actual.selected, { key: 'key-2', value: 'value-2' });
  });
  test('it turns arrays with only one element into key values and selects the defined index', function(assert) {
    const actual = selectableKeyValues([['Value 1'], ['Value 2']], { selected: 1 });
    assert.equal(actual.items.length, 2);
    assert.deepEqual(actual.selected, { key: 'value-2', value: 'Value 2' });
  });
  test('it turns strings into key values and selects the defined index', function(assert) {
    const actual = selectableKeyValues(['Value 1', 'Value 2'], { selected: 1 });
    assert.equal(actual.items.length, 2);
    assert.deepEqual(actual.selected, { key: 'value-2', value: 'Value 2' });
  });
});
