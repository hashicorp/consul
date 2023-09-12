import httpRequest from 'consul-ui/utils/http/request';
import { module, test } from 'qunit';

module('Unit | Utility | http/request', function () {
  // Replace this with your real tests.
  test('it works', function (assert) {
    const actual = httpRequest;
    assert.ok(typeof actual === 'function');
  });
});
