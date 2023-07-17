import wildcard from 'consul-ui/utils/routing/wildcard';
import { module, test } from 'qunit';

module('Unit | Utility | routing/wildcard', function () {
  test('it finds a * in a path', function (assert) {
    const isWildcard = wildcard({
      route: {
        _options: {
          path: 'i-m-a-wildcard*',
        },
      },
    });
    assert.ok(isWildcard('route'));
  });
  test("it returns false without throwing if it doesn't find route", function (assert) {
    const isWildcard = wildcard({
      route: {
        _options: {
          path: 'i-m-a-wildcard*',
        },
      },
    });
    assert.notOk(isWildcard('not-route'));
  });
});
