import templatize from 'consul-ui/utils/templatize';
import { module, test } from 'qunit';

module('Unit | Utility | templatize');

test('it prefixes the word template to every string in the array', function(assert) {
  const expected = ['template-one', 'template-two'];
  const actual = templatize(['one', 'two']);
  assert.deepEqual(actual, expected);
});
test('it returns an empty array when passed an empty array', function(assert) {
  const expected = [];
  const actual = templatize([]);
  assert.deepEqual(actual, expected);
});
test('it returns an empty array when passed nothing', function(assert) {
  const expected = [];
  const actual = templatize();
  assert.deepEqual(actual, expected);
});
