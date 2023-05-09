import { andOr } from 'consul-ui/utils/filter';
import predicates from 'consul-ui/filter/predicates/intention';
import { module, test } from 'qunit';

module('Unit | Filter | Predicates | intention', function () {
  const predicate = andOr(predicates);

  test('it returns items depending on Action', function (assert) {
    const items = [
      {
        Action: 'allow',
      },
      {
        Action: 'deny',
      },
    ];

    let expected, actual;

    expected = [items[0]];
    actual = items.filter(
      predicate({
        access: ['allow'],
      })
    );
    assert.deepEqual(actual, expected);

    expected = [items[1]];
    actual = items.filter(
      predicate({
        access: ['deny'],
      })
    );
    assert.deepEqual(actual, expected);

    expected = items;
    actual = items.filter(
      predicate({
        access: ['allow', 'deny'],
      })
    );
    assert.deepEqual(actual, expected);
  });
});
