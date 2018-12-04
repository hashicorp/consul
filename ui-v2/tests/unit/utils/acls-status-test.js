import { module } from 'ember-qunit';
import test from 'ember-sinon-qunit/test-support/test';
import aclsStatus from 'consul-ui/utils/acls-status';

module('Unit | Utility | acls status');

test('it rejects and nothing is enabled or authorized', function(assert) {
  const isValidServerError = this.stub().returns(false);
  const status = aclsStatus(isValidServerError);
  [
    this.stub().rejects(),
    this.stub().rejects({ errors: [] }),
    this.stub().rejects({ errors: [{ status: '404' }] }),
  ].forEach(function(reject) {
    const actual = status({
      response: reject(),
    });
    assert.rejects(actual.response);
    ['isAuthorized', 'isEnabled'].forEach(function(prop) {
      actual[prop].then(function(actual) {
        assert.notOk(actual);
      });
    });
  });
});
test('with a 401 it resolves with an empty array and nothing is enabled or authorized', function(assert) {
  assert.expect(3);
  const isValidServerError = this.stub().returns(false);
  const status = aclsStatus(isValidServerError);
  const actual = status({
    response: this.stub().rejects({ errors: [{ status: '401' }] })(),
  });
  actual.response.then(function(actual) {
    assert.deepEqual(actual, []);
  });
  ['isAuthorized', 'isEnabled'].forEach(function(prop) {
    actual[prop].then(function(actual) {
      assert.notOk(actual);
    });
  });
});
test("with a 403 it resolves with an empty array and it's enabled but not authorized", function(assert) {
  assert.expect(3);
  const isValidServerError = this.stub().returns(false);
  const status = aclsStatus(isValidServerError);
  const actual = status({
    response: this.stub().rejects({ errors: [{ status: '403' }] })(),
  });
  actual.response.then(function(actual) {
    assert.deepEqual(actual, []);
  });
  actual.isEnabled.then(function(actual) {
    assert.ok(actual);
  });
  actual.isAuthorized.then(function(actual) {
    assert.notOk(actual);
  });
});
test("with a 500 (but not a 'valid' error) it rejects and nothing is enabled or authorized", function(assert) {
  assert.expect(3);
  const isValidServerError = this.stub().returns(false);
  const status = aclsStatus(isValidServerError);
  const actual = status({
    response: this.stub().rejects({ errors: [{ status: '500' }] })(),
  });
  assert.rejects(actual.response);
  ['isAuthorized', 'isEnabled'].forEach(function(prop) {
    actual[prop].then(function(actual) {
      assert.notOk(actual);
    });
  });
});
test("with a 500 and a 'valid' error, it resolves with an empty array and it's enabled but not authorized", function(assert) {
  assert.expect(3);
  const isValidServerError = this.stub().returns(true);
  const status = aclsStatus(isValidServerError);
  const actual = status({
    response: this.stub().rejects({ errors: [{ status: '500' }] })(),
  });
  actual.response.then(function(actual) {
    assert.deepEqual(actual, []);
  });
  actual.isEnabled.then(function(actual) {
    assert.ok(actual);
  });
  actual.isAuthorized.then(function(actual) {
    assert.notOk(actual);
  });
});
