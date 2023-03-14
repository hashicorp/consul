/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import httpXhr from 'consul-ui/utils/http/xhr';
import { module, test } from 'qunit';

module('Unit | Utility | http/xhr', function () {
  // Replace this with your real tests.
  test('it works', function (assert) {
    let result = httpXhr();
    assert.ok(result);
  });
});
