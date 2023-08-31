import { module, test } from 'qunit';
import Helper from 'consul-ui/helpers/document-attrs';

const root = {
  classList: {
    add: () => {},
    remove: () => {},
  },
};
module('Unit | Helper | document-attrs', function () {
  test('synchronize adds and removes values correctly', function (assert) {
    let attrs, actual;
    // add first helper
    const a = new Helper();
    attrs = a.synchronize(root, {
      class: 'a b a a a a',
    });
    actual = [...attrs.get('class').keys()];
    assert.deepEqual(actual, ['a', 'b'], 'keys are adding correctly');
    const b = new Helper();
    // add second helper
    attrs = b.synchronize(root, {
      class: 'z a a a a',
    });
    actual = [...attrs.get('class').keys()];
    assert.deepEqual(actual, ['a', 'b', 'z'], 'more keys are added correctly');
    // remove second helper
    b.synchronize(root);
    actual = [...attrs.get('class').keys()];
    assert.deepEqual(actual, ['a', 'b'], 'keys are removed, leaving keys that need to remain');
    // remove first helper
    a.synchronize(root);
    assert.strictEqual(
      typeof attrs.get('class'),
      'undefined',
      'property is completely removed once its empty'
    );
    assert.throws(() => {
      a.synchronize(root, { data: 'a' });
    }, `throws an error if the attrs isn't class`);
  });
});
