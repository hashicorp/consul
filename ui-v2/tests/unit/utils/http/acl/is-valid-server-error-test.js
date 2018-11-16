import createIsValidServerError from 'consul-ui/utils/http/acl/is-valid-server-error';
import { module, test } from 'qunit';

module('Unit | Utility | http/acl/is valid server error');
const createEmberDataError = function(response) {
  return {
    errors: [
      {
        detail: response,
      },
    ],
  };
};
test('it returns a function', function(assert) {
  const isValidServerError = createIsValidServerError();
  assert.ok(typeof isValidServerError === 'function');
});
test("it returns false if there is no 'correctly' formatted error", function(assert) {
  const isValidServerError = createIsValidServerError();
  assert.notOk(isValidServerError());
  assert.notOk(isValidServerError({}));
  assert.notOk(isValidServerError({ errors: {} }));
  assert.notOk(isValidServerError({ errors: [{}] }));
  assert.notOk(isValidServerError({ errors: [{ notDetail: '' }] }));
});
// don't go too crazy with these, just enough for sanity check, we are essentially testing indexOf
test("it returns false if the response doesn't contain the exact error response", function(assert) {
  const isValidServerError = createIsValidServerError();
  [
    "pc error making call: rpc: can't find method ACL",
    "rpc error making call: rpc: can't find method",
    "rpc rror making call: rpc: can't find method ACL",
  ].forEach(function(response) {
    const e = createEmberDataError(response);
    assert.notOk(isValidServerError(e));
  });
});
test('it returns true if the response contains the exact error response', function(assert) {
  const isValidServerError = createIsValidServerError();
  [
    "rpc error making call: rpc: can't find method ACL",
    " rpc error making call: rpc: can't find method ACL",
    "rpc error making call: rpc: rpc error making call: rpc: rpc error making call: rpc: can't find method ACL",
  ].forEach(function(response) {
    const e = createEmberDataError(response);
    assert.ok(isValidServerError(e));
  });
});
