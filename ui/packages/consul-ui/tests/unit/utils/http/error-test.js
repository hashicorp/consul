/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import HttpError from 'consul-ui/utils/http/error';
import { module, test } from 'qunit';

module('Unit | Utility | http/error', function () {
  // Replace this with your real tests.
  test('it works', function (assert) {
    const result = new HttpError();
    assert.ok(result);
  });
});
