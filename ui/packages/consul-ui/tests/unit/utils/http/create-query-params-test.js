/**
 * Copyright IBM Corp. 2014, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import createQueryParams from 'consul-ui/utils/http/create-query-params';
import { module, test } from 'qunit';

module('Unit | Utility | http/create-query-params', function () {
  const stringifyQueryParams = createQueryParams((str) => str);
  test('it turns objects into query params formatted strings', function (assert) {
    const expected = 'something=here&another=variable';
    const actual = stringifyQueryParams({
      something: 'here',
      another: 'variable',
    });
    assert.strictEqual(actual, expected);
  });
  test('it ignores undefined properties', function (assert) {
    const expected = 'something=here';
    const actual = stringifyQueryParams({
      something: 'here',
      another: undefined,
    });
    assert.strictEqual(actual, expected);
  });
  test('it stringifies nested objects', function (assert) {
    const expected = 'something=here&another[something]=here&another[another][something]=here';
    const actual = stringifyQueryParams({
      something: 'here',
      another: {
        something: 'here',
        another: {
          something: 'here',
        },
      },
    });
    assert.strictEqual(actual, expected);
  });
  test('it only adds the property if the value is null', function (assert) {
    const expected = 'something&another=here';
    const actual = stringifyQueryParams({
      something: null,
      another: 'here',
    });
    assert.strictEqual(actual, expected);
  });
});
