/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import createHeaders from 'consul-ui/utils/http/create-headers';
import { module, test } from 'qunit';

module('Unit | Utility | http/create-headers', function () {
  const parseHeaders = createHeaders();
  test('it converts lines of header-like strings into an object', function (assert) {
    const expected = {
      'Content-Type': 'application/json',
      'X-Consul-Index': '1',
    };
    const lines = `
      Content-Type: application/json
      X-Consul-Index: 1
    `.split('\n');
    const actual = parseHeaders(lines);
    assert.deepEqual(actual, expected);
  });
  test('it parses header values with colons correctly', function (assert) {
    const expected = {
      'Content-Type': 'application/json',
      'X-Consul-Index': '1:2:3',
    };
    const lines = `
      Content-Type: application/json
      X-Consul-Index: 1:2:3
    `.split('\n');
    const actual = parseHeaders(lines);
    assert.deepEqual(actual, expected);
  });
});
