import { module, skip } from 'ember-qunit';
import test from 'ember-sinon-qunit/test-support/test';
import createURL from 'consul-ui/utils/createURL';
module('Unit | Utils | createURL', {});

skip("it isn't isolated enough, mock encodeURIComponent");
test('it passes the values to encode', function(assert) {
  const url = createURL(encodeURIComponent);
  const actual = url`/v1/url?${{ query: 'to encode', 'key with': ' spaces ' }}`;
  const expected = '/v1/url?query=to%20encode&key%20with=%20spaces%20';
  assert.equal(actual, expected);
});
test('it adds a query string key without an `=` if the query value is `null`', function(assert) {
  const url = createURL(encodeURIComponent);
  const actual = url`/v1/url?${{ 'key with space': null }}`;
  const expected = '/v1/url?key%20with%20space';
  assert.equal(actual, expected);
});
test('it returns a string when passing an array', function(assert) {
  const url = createURL(encodeURIComponent);
  const actual = url`/v1/url/${['raw values', 'to', 'encode']}`;
  const expected = '/v1/url/raw%20values/to/encode';
  assert.equal(actual, expected);
});
test('it returns a string when passing a string', function(assert) {
  const url = createURL(encodeURIComponent);
  const actual = url`/v1/url/${'raw values to encode'}`;
  const expected = '/v1/url/raw%20values%20to%20encode';
  assert.equal(actual, expected);
});
test("it doesn't add a query string prop/value is the value is undefined", function(assert) {
  const url = createURL(encodeURIComponent);
  const actual = url`/v1/url?${{ key: undefined }}`;
  const expected = '/v1/url?';
  assert.equal(actual, expected);
});
