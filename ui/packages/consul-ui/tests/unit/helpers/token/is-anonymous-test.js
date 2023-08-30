import { isAnonymous } from 'consul-ui/helpers/token/is-anonymous';
import { module, test } from 'qunit';

module('Unit | Helper | token/is-anonymous', function () {
  test('it returns true if the token is the anonymous token', function (assert) {
    const actual = isAnonymous([{ AccessorID: '00000000-0000-0000-0000-000000000002' }]);
    assert.ok(actual);
  });
  test("it returns false if the token isn't the anonymous token", function (assert) {
    const actual = isAnonymous([{ AccessorID: '00000000-0000-0000-0000-000000000000' }]);
    assert.notOk(actual);
  });
});
