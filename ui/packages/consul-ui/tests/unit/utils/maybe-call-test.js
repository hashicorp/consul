/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import maybeCall from 'consul-ui/utils/maybe-call';
import { module, test } from 'qunit';
import { Promise } from 'rsvp';

module('Unit | Utility | maybe-call', function () {
  test('it calls a function when the resolved value is true', async function (assert) {
    let called = false;
    await maybeCall(() => {
      called = true;
    }, Promise.resolve(true))();
    assert.true(called, 'callback was called');
  });

  test("it doesn't call a function when the resolved value is false", async function (assert) {
    let called = false;
    await maybeCall(() => {
      called = true;
    }, Promise.resolve(false))();
    assert.false(called, 'callback was not called');
  });
});
