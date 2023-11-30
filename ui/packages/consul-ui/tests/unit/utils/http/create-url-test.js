/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, skip, test } from 'qunit';
import createURL from 'consul-ui/utils/http/create-url';
import createQueryParams from 'consul-ui/utils/http/create-query-params';

module('Unit | Utils | http/create-url', function () {
  skip("it isn't isolated enough, mock encodeURIComponent");
  const url = createURL(encodeURIComponent, createQueryParams(encodeURIComponent));
  test('it passes the values to encode', function (assert) {
    const actual = url`/v1/url?${{ query: 'to encode', 'key with': ' spaces ' }}`;
    const expected = '/v1/url?query=to%20encode&key%20with=%20spaces%20';
    assert.equal(actual, expected);
  });
  test('it adds a query string key without an `=` if the query value is `null`', function (assert) {
    const actual = url`/v1/url?${{ 'key with space': null }}`;
    const expected = '/v1/url?key%20with%20space';
    assert.equal(actual, expected);
  });
  test('it returns a string when passing an array', function (assert) {
    const actual = url`/v1/url/${['raw values', 'to', 'encode']}`;
    const expected = '/v1/url/raw%20values/to/encode';
    assert.equal(actual, expected);
  });
  test('it returns a string when passing a string', function (assert) {
    const actual = url`/v1/url/${'raw values to encode'}`;
    const expected = '/v1/url/raw%20values%20to%20encode';
    assert.equal(actual, expected);
  });
  test("it doesn't add a query string prop/value is the value is undefined", function (assert) {
    const actual = url`/v1/url?${{ key: undefined }}`;
    const expected = '/v1/url?';
    assert.equal(actual, expected);
  });
  test("it doesn't encode headers", function (assert) {
    const actual = url`
      /v1/url/${'raw values to encode'}
      Header: %value
    `;
    const expected = `/v1/url/raw%20values%20to%20encode
      Header: %value`;
    assert.equal(actual, expected);
  });
});
